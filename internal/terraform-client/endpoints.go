package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// CreateSession creates an endpoint (session) and waits for it to become
// administratively ready (created or paused). Readiness failures return both
// the assigned identity and an error so Terraform can recover the recorded row.
// This does not verify runtime connectivity or policy enforcement.
func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (*CreateSessionResponse, error) {
	tflog.Debug(ctx, "CreateSession called", map[string]interface{}{
		"name":          req.SessionName,
		"resource":      req.ResourceName,
		"authorization": req.AuthorizationName,
		"cluster":       req.ClusterName,
		"type":          req.SessionType,
	})

	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.sessionAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	_response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer _response.Body.Close()

	if _response.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("duplicate session with name %s", req.SessionName)
	}
	if _response.StatusCode != http.StatusOK {
		errReason := readBody(_response)
		tflog.Error(ctx, "Failed to create session", map[string]interface{}{
			"name":        req.SessionName,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error creating session %s, reason %s", req.SessionName, errReason)
	}

	var response CreateSessionResponse
	if err := decodeResponse(_response.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}

	// Creation is not readiness: retain the ID and fail explicitly when the
	// recorded endpoint cannot be confirmed ready within the caller's budget.
	if _, err := c.waitForSession(ctx, response.ID); err != nil {
		tflog.Error(ctx, "Failed to verify session after creation", map[string]interface{}{
			"id":    response.ID,
			"error": err.Error(),
		})
		return &response, fmt.Errorf("endpoint recorded but readiness unconfirmed: %w", err)
	}
	tflog.Debug(ctx, "Session successfully created", map[string]interface{}{
		"id":   response.ID,
		"name": req.SessionName,
	})
	return &response, nil
}

// ReadSession fetches the current state of an endpoint (session) by ID. It
// returns (nil, nil) when the session no longer exists so callers can drop it
// from Terraform state.
func (c *Client) ReadSession(ctx context.Context, sessionID string) (*SessionReadResponse, error) {
	return readTyped[SessionReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.sessionAPI(), sessionID), "session", sessionID)
}

// waitForSession polls creating/restarting until created or paused. A
// not-found read is retried: right after create the row may not be readable yet.
func (c *Client) waitForSession(ctx context.Context, sessionID string) (*SessionReadResponse, error) {
	return DoContext(ctx,
		func() (*SessionReadResponse, error) {
			r, err := c.ReadSession(ctx, sessionID)
			if err != nil {
				return nil, err
			}
			if r != nil {
				switch strings.ToLower(r.Status) {
				case "created", "paused", "creating", "restarting":
				default:
					return nil, fmt.Errorf("endpoint readiness failed (status not ready); object remains recorded")
				}
			}
			return r, nil
		}, RetryLimit(30), Sleep(c.pollInterval), RetryResultChecker(func(intermedResult any) bool {
			res, ok := intermedResult.(*SessionReadResponse)
			if !ok || res == nil {
				return true
			}
			// Only transient provisioning statuses continue polling.
			return strings.ToLower(res.Status) == "creating" || strings.ToLower(res.Status) == "restarting"
		}))
}

func (c *Client) UpdateSession(ctx context.Context, sessionID string, req CreateSessionRequest) (*UpdateSessionResponse, error) {
	tflog.Debug(ctx, "UpdateSession called", map[string]interface{}{
		"session_id": sessionID,
		"name":       req.SessionName,
		"resource":   req.ResourceName,
	})

	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.sessionAPI(), sessionID), payloadBuf)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	_response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer _response.Body.Close()

	if _response.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("duplicate session with name %s", req.SessionName)
	}
	if _response.StatusCode != http.StatusOK {
		errReason := readBody(_response)
		tflog.Error(ctx, "Failed to update session", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error updating session %s, reason %s", req.SessionName, errReason)
	}

	var response UpdateSessionResponse
	if err := decodeResponse(_response.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Session successfully updated", map[string]interface{}{
		"session_id": sessionID,
	})
	return &response, nil
}

// isSessionReaped reports whether a session status means the session has
// stopped and its row can be force-deleted.
func isSessionReaped(status string) bool {
	switch strings.ToLower(status) {
	case "terminated", "marked-for-deletion", "failed", "failed-to-restart":
		return true
	}
	return false
}

// DeleteSession terminates an endpoint's session and then hard-deletes its row.
//
// Terminating alone leaves the sessions row (and its integration association)
// behind, which keeps the parent resource undeletable until the deferred
// reaper runs. The force delete needs the terminal status the first step
// establishes, so we wait for it before issuing it. Row absence establishes
// configuration deletion only; asynchronous Kubernetes cleanup is backend-owned.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) (bool, error) {
	tflog.Debug(ctx, "DeleteSession called", map[string]interface{}{
		"session_id": sessionID,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		// This backend returns a 200 string when termination finds no session.
		// Only an independent read 404 establishes idempotent absence.
		var statusErr *HTTPError
		if errors.As(err, &statusErr) && statusErr.StatusCode == 200 && statusErr.Reason == "error while terminating session" {
			if r, readErr := c.ReadSession(ctx, sessionID); readErr == nil && r == nil {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to request adaptive api: %w", err)
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return true, nil
	}
	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		response.Body.Close()
		tflog.Error(ctx, "Failed to initiate session deletion", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		return false, fmt.Errorf("error deleting session %s: %s", sessionID, body)
	}
	response.Body.Close()

	// Wait for the session to reach a terminal status. A read that 404s means
	// the row is already gone, which is equally terminal.
	resp, err := DoContext(ctx,
		func() (*SessionReadResponse, error) {
			r, err := c.ReadSession(ctx, sessionID)
			if err != nil {
				return nil, err
			}
			if r != nil && !isSessionReaped(r.Status) {
				switch strings.ToLower(r.Status) {
				case "created", "creating", "restarting", "terminating", "paused", "pausing", "unhealthy":
				default:
					return nil, fmt.Errorf("endpoint deletion cannot proceed from reported status")
				}
			}
			return r, nil
		}, RetryLimit(20), Sleep(c.pollInterval), RetryResultChecker(func(intermedResult any) bool {
			res, ok := intermedResult.(*SessionReadResponse)
			if !ok {
				return true
			}
			if res == nil {
				return false // already gone
			}
			return !isSessionReaped(res.Status)
		}))
	if err != nil {
		tflog.Error(ctx, "Failed to read session during deletion", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, fmt.Errorf("could not read session %s while deleting: %w", sessionID, err)
	}
	if resp == nil {
		tflog.Debug(ctx, "Session already removed", map[string]interface{}{"session_id": sessionID})
		return true, nil
	}

	// Hard-delete the row so the resource it points at can be deleted right away.
	if _, err := c.forceDeleteSession(ctx, sessionID); err != nil {
		// The backend reports a missing row as 500 here. Only an independent
		// authoritative read 404 can make that disappearance race idempotent.
		var statusErr *HTTPError
		if errors.As(err, &statusErr) && statusErr.StatusCode == 500 && statusErr.Reason == "failed to get session" {
			if r, readErr := c.ReadSession(ctx, sessionID); readErr == nil && r == nil {
				return true, nil
			}
		}
		return false, err
	}
	tflog.Debug(ctx, "Session successfully deleted", map[string]interface{}{
		"session_id": sessionID,
		"status":     resp.Status,
	})
	return true, nil
}

func (c *Client) forceDeleteSession(ctx context.Context, sessionID string) (bool, error) {
	tflog.Debug(ctx, "Force deleting session", map[string]interface{}{
		"session_id": sessionID,
	})

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/forcedelete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return false, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	defer _response.Body.Close()

	if _response.StatusCode == http.StatusNotFound {
		return true, nil
	}
	if _response.StatusCode != http.StatusOK {
		body := readBody(_response)
		tflog.Error(ctx, "Force delete failed", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": _response.StatusCode,
			"reason":      body,
		})
		return false, fmt.Errorf("error force deleting session %s: %s", sessionID, body)
	}

	tflog.Debug(ctx, "Session force deleted successfully", map[string]interface{}{
		"session_id": sessionID,
	})
	return true, nil
}
