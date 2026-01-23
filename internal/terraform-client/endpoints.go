package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Sessions
func (c *Client) CreateSession(
	ctx context.Context,
	sessionName,
	resourceName,
	authorizationName,
	clusterName,
	ttl,
	sessionType string,
	isJITEnabled bool,
	accessApprovers []string,
	pauseTimeout string,
	users []string,
	memory, cpu string,
	tags []string,
	groups []string,
	idleTimeout string,
) (*CreateSessionResponse, error) {
	tflog.Debug(ctx, "CreateSession called", map[string]interface{}{
		"name":          sessionName,
		"resource":      resourceName,
		"authorization": authorizationName,
		"cluster":       clusterName,
		"type":          sessionType,
	})
	req := CreateSessionRequest{
		SessionName:       sessionName,
		ResourceName:      resourceName,
		ClusterName:       clusterName,
		AuthorizationName: authorizationName,
		SessionTTL:        ttl,
		SessionType:       sessionType,
		SessionUsers:      users,
		IsJITEnabled:      isJITEnabled,
		AccessApprovers:   accessApprovers,
		PauseTimeout:      pauseTimeout,
		IdleTimeout:       idleTimeout,
		Memory:            memory,
		CPU:               cpu,
		UsersTags:         tags,
		Groups:            groups,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for creating session", map[string]interface{}{
			"name":  sessionName,
			"error": err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.sessionAPI()), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for creating session", map[string]interface{}{
			"name":  sessionName,
			"error": err.Error(),
		})
		return nil, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for creating session", map[string]interface{}{
			"name":  sessionName,
			"error": err.Error(),
		})
		return nil, err
	}
	if _response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate session detected", map[string]interface{}{
			"name": sessionName,
		})
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}

	if _response.StatusCode != 200 {
		errReason, err := io.ReadAll(_response.Body)
		if err != nil {
			tflog.Error(ctx, "Failed to read error response body for session", map[string]interface{}{
				"name":       req.SessionName,
				"read_error": err.Error(),
			})
			return nil, fmt.Errorf("error decoding response %s", err)
		}
		tflog.Error(ctx, "Failed to create session", map[string]interface{}{
			"name":        req.SessionName,
			"status_code": _response.StatusCode,
			"reason":      string(errReason),
		})
		return nil, fmt.Errorf("error creating session %s, reason %s", req.SessionName, string(errReason))
	}

	var response CreateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		tflog.Error(ctx, "Failed to decode success response body for session", map[string]interface{}{
			"name":  req.SessionName,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Session created, waiting for it to be functional", map[string]interface{}{
		"id": response.ID,
	})
	// now wait for it to be functional
	if _, err := c.ReadSession(ctx, response.ID, true); err != nil {
		tflog.Error(ctx, "Failed to verify session after creation", map[string]interface{}{
			"id":    response.ID,
			"error": err.Error(),
		})
		// return &response, fmt.Errorf("failed to create session %s", response.ID)
	}
	tflog.Debug(ctx, "Session successfully created and verified", map[string]interface{}{
		"id":   response.ID,
		"name": sessionName,
	})
	return &response, nil
}

func _readSession(ctx context.Context, c *Client, sessionID string) (map[string]interface{}, error) {
	tflog.Debug(ctx, "Reading session", map[string]interface{}{
		"session_id": sessionID,
	})
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for reading session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to request adaptive api for session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		tflog.Error(ctx, "Unexpected status code reading session", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": response.StatusCode,
			"expected":    http.StatusAccepted,
		})
		return nil, fmt.Errorf("error read session %s", sessionID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Successfully read session", map[string]interface{}{
		"session_id": sessionID,
		"response":   fmt.Sprintf("%+v", resp),
	})
	return resp, nil
}

/*
waitForStatus: if true, will wait for session to be active/fail before returning
*/
func (c *Client) ReadSession(ctx context.Context, sessionID string, waitForStatus bool) (map[string]interface{}, error) {
	tflog.Debug(ctx, "ReadSession called", map[string]interface{}{
		"session_id":      sessionID,
		"wait_for_status": waitForStatus,
	})
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 30
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(ctx, c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			tflog.Debug(ctx, "Session status check", map[string]interface{}{
				"result": fmt.Sprintf("%v", intermedResult),
			})
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				tflog.Warn(ctx, "Session result has bad data format", map[string]interface{}{
					"session_id": sessionID,
				})
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						tflog.Warn(ctx, "Session status field missing or not a string", map[string]interface{}{
							"session_id": sessionID,
						})
						return true
					} else {
						statusLower := strings.ToLower(status)
						tflog.Debug(ctx, "Session status check", map[string]interface{}{
							"session_id":  sessionID,
							"status":      status,
							"is_creating": statusLower == "creating",
						})
						// Will return false, when state is among final states like "created" or "failed"
						return statusLower == "creating"
					}
				}
				tflog.Warn(ctx, "Session result is nil", map[string]interface{}{
					"session_id": sessionID,
				})
				return true
			}
		}))
	if err != nil {
		tflog.Error(ctx, "Failed to read session after retries", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("could to create session %s", sessionID)
	}
	finalStatus := strings.ToLower(resp["Status"].(string))
	if finalStatus != "created" {
		tflog.Error(ctx, "Session not in created status", map[string]interface{}{
			"session_id": sessionID,
			"status":     resp["Status"],
		})
		return nil, fmt.Errorf("error create session %s", sessionID)
	}
	tflog.Debug(ctx, "Session successfully created", map[string]interface{}{
		"session_id": sessionID,
	})
	return resp, nil
}

func (c *Client) UpdateSession(
	ctx context.Context,
	sessionID,
	sessionName,
	resourceName,
	authorizationName,
	clusterName,
	ttl,
	sessionType string,
	isJITEnabled bool,
	accessApprovers []string,
	pauseTimeout string,
	users []string,
	memory, cpu string,
	tags []string,
	groups []string,
	idleTimeout string,
) (*UpdateSessionResponse, error) {
	tflog.Debug(ctx, "UpdateSession called", map[string]interface{}{
		"session_id": sessionID,
		"name":       sessionName,
		"resource":   resourceName,
	})
	req := UpdateSessionRequest{
		SessionName:       sessionName,
		ResourceName:      resourceName,
		ClusterName:       clusterName,
		SessionType:       sessionType,
		AuthorizationName: authorizationName,
		SessionTTL:        ttl,
		SessionUsers:      users,
		IsJITEnabled:      isJITEnabled,
		AccessApprovers:   accessApprovers,
		PauseTimeout:      pauseTimeout,
		Memory:            memory,
		CPU:               cpu,
		UsersTags:         tags,
		Groups:            groups,
		IdleTimeout:       idleTimeout,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for updating session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.sessionAPI(), sessionID), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for updating session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for updating session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, err
	}
	if _response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate session name during update", map[string]interface{}{
			"session_id": sessionID,
			"name":       sessionName,
		})
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			tflog.Error(ctx, "Failed to decode error response for session update", map[string]interface{}{
				"session_id":   sessionID,
				"decode_error": decodeErr.Error(),
			})
		}
		tflog.Error(ctx, "Failed to update session", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error updating session %s, reason %s", req.SessionName, errReason)
	}
	var response UpdateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		tflog.Error(ctx, "Failed to decode success response body for session update", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Session successfully updated", map[string]interface{}{
		"session_id": sessionID,
	})
	return &response, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) (bool, error) {
	tflog.Debug(ctx, "DeleteSession called", map[string]interface{}{
		"session_id": sessionID,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for deleting session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for deleting session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to initiate session deletion", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": response.StatusCode,
		})
		return false, fmt.Errorf("error deleting session %s", sessionID)
	}
	tflog.Debug(ctx, "Delete request successful, monitoring session termination", map[string]interface{}{
		"session_id": sessionID,
	})
	// Once delete request is succesful, we check for status of session
	timeout := time.Second * 10
	retryForStatus := 20

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(ctx, c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				tflog.Warn(ctx, "Session deletion check has bad data format", map[string]interface{}{
					"session_id": sessionID,
				})
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						tflog.Warn(ctx, "Session status field missing during deletion", map[string]interface{}{
							"session_id": sessionID,
						})
						return false
					} else {
						statusLower := strings.ToLower(status)
						tflog.Debug(ctx, "Session deletion status check", map[string]interface{}{
							"session_id": sessionID,
							"status":     status,
						})

						// Return true to keep retrying if NOT in a terminal state
						terminatedOrFailed := (statusLower == "terminated" || statusLower == "marked-for-deletion" || statusLower == "failed" || statusLower == "failed-to-restart")
						if terminatedOrFailed {
							tflog.Debug(ctx, "Session in terminal state, forcing deletion", map[string]interface{}{
								"session_id": sessionID,
								"status":     status,
							})
							_, err2 := c.deleteSession(ctx, sessionID)
							if err2 != nil {
								tflog.Error(ctx, "Failed to force delete session", map[string]interface{}{
									"session_id": sessionID,
									"error":      err2.Error(),
								})
								return false
							}
							return false
						}
					}
				}
				return true
			}
		}))
	if err != nil {
		tflog.Error(ctx, "Failed to read session during deletion", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, fmt.Errorf("could to read session %s %w", sessionID, err)
	}

	if status, ok := resp["Status"].(string); ok {
		statusLower := strings.ToLower(status)
		tflog.Debug(ctx, "Final session deletion status", map[string]interface{}{
			"session_id": sessionID,
			"status":     status,
		})
		if statusLower == "terminated" || statusLower == "marked-for-deletion" || statusLower == "does-not-exist" {
			tflog.Debug(ctx, "Session successfully deleted", map[string]interface{}{
				"session_id": sessionID,
			})
			return true, nil
		}
		tflog.Error(ctx, "Unexpected session status after deletion", map[string]interface{}{
			"session_id": sessionID,
			"status":     statusLower,
		})
		return false, fmt.Errorf("error read session %s", sessionID)
	} else {
		tflog.Error(ctx, "Could not determine status for session", map[string]interface{}{
			"session_id": sessionID,
		})
		return false, errors.New("could not delete session")
	}

	return true, nil
}

func (c *Client) deleteSession(ctx context.Context, sessionID string) (bool, error) {
	tflog.Debug(ctx, "Force deleting session", map[string]interface{}{
		"session_id": sessionID,
	})

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/forcedelete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for force deleting session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for force deleting session", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}

	if _response.StatusCode != 200 {
		tflog.Error(ctx, "Force delete failed", map[string]interface{}{
			"session_id":  sessionID,
			"status_code": _response.StatusCode,
		})
		return false, fmt.Errorf("error force deleting session %s", sessionID)
	}

	tflog.Debug(ctx, "Session force deleted successfully", map[string]interface{}{
		"session_id": sessionID,
	})
	return true, nil
}
