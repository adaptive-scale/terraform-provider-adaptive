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
	scriptOnlyAccess bool,
	policies ...SessionPolicy,
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
		ScriptOnlyAccess:  scriptOnlyAccess,
	}

	if len(policies) > 0 {
		req.SessionPolicy = policies[0]
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
	defer _response.Body.Close()
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

/*
waitForStatus: if true, will wait for session to be active/fail before returning
*/
func (c *Client) ReadSession(ctx context.Context, sessionID string, waitForStatus bool) (map[string]interface{}, error) {
	return c.readWithStatus(ctx, c.sessionAPI(), sessionID, waitForStatus)
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
	scriptOnlyAccess bool,
	policies ...SessionPolicy,
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
		ScriptOnlyAccess:  scriptOnlyAccess,
	}

	if len(policies) > 0 {
		req.SessionPolicy = policies[0]
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
	defer _response.Body.Close()
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
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return false, err
	}
	response, err := c.do(ctx, request)
	if err != nil {
		return false, err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("error deleting session %s: HTTP %d", sessionID, response.StatusCode)
	}
	for attempt := 0; attempt < 20; attempt++ {
		data, err := c.ReadSession(ctx, sessionID, false)
		if errors.Is(err, ErrNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		value, _ := ResponseValue(data, "status", "Status")
		status, ok := value.(string)
		if !ok {
			return false, fmt.Errorf("missing session deletion status for %s", sessionID)
		}
		switch strings.ToLower(status) {
		case "does-not-exist":
			return true, nil
		case "terminated", "marked-for-deletion", "failed", "failed-to-restart":
			return c.deleteSession(ctx, sessionID)
		}
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, fmt.Errorf("timed out deleting session %s", sessionID)
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
	defer _response.Body.Close()

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
