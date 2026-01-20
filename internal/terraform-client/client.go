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

const (
	defaultAdaptiveURL = "https://app.adaptive.com/api/v1"
)

type Client struct {
	serviceToken string
	workspaceURL string
	httpClient   *http.Client
}

func NewClient(serviceToken, workspaceURL string) *Client {

	if workspaceURL == "" {
		workspaceURL = defaultAdaptiveURL
	} else {
		workspaceURL = fmt.Sprintf("%s/api/v1", workspaceURL)
	}

	return &Client{
		serviceToken: serviceToken,
		workspaceURL: workspaceURL,
		httpClient:   &http.Client{},
	}
}

func (c *Client) authorizationAPI() string {
	return fmt.Sprintf("%s/terraform/authorization", c.workspaceURL)
}

func (c *Client) teamAPI() string {
	return fmt.Sprintf("%s/terraform/team", c.workspaceURL)
}

func (c *Client) scriptAPI() string {
	return fmt.Sprintf("%s/terraform/script", c.workspaceURL)
}

func (c *Client) resourceAPI() string {
	return fmt.Sprintf("%s/terraform/resource", c.workspaceURL)
}

func (c *Client) sessionAPI() string {
	return fmt.Sprintf("%s/terraform/session", c.workspaceURL)
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	tflog.Debug(ctx, "Making HTTP request", map[string]interface{}{
		"method": req.Method,
		"url":    req.URL.String(),
	})
	req.Header.Set("Authorization", c.serviceToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to make HTTP request", map[string]interface{}{
			"method": req.Method,
			"url":    req.URL.String(),
			"error":  err.Error(),
		})
		return nil, err
	}

	tflog.Debug(ctx, "HTTP response received", map[string]interface{}{
		"status_code": res.StatusCode,
		"url":         req.URL.String(),
	})
	if res.StatusCode == 401 {
		tflog.Error(ctx, "Authentication failed: bad token", map[string]interface{}{
			"url": req.URL.String(),
		})
		return nil, errors.New("bad token. please check your service token")
	}
	return res, err
}

func _readAuthorization(ctx context.Context, c *Client, authID string) (map[string]interface{}, error) {
	tflog.Debug(ctx, "Reading authorization", map[string]interface{}{
		"auth_id": authID,
	})
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for reading authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to request adaptive api for authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		tflog.Error(ctx, "Unexpected status code reading authorization", map[string]interface{}{
			"auth_id":     authID,
			"status_code": response.StatusCode,
			"expected":    http.StatusAccepted,
		})
		return nil, fmt.Errorf("error read authorization %s", authID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Successfully read authorization", map[string]interface{}{
		"auth_id":  authID,
		"response": fmt.Sprintf("%+v", resp),
	})
	return resp, nil
}

func (c *Client) ReadAuthorization(ctx context.Context, authID string, waitForStatus bool) (any, error) {
	tflog.Debug(ctx, "ReadAuthorization called", map[string]interface{}{
		"auth_id":         authID,
		"wait_for_status": waitForStatus,
	})
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 20
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readAuthorization(ctx, c, authID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				tflog.Warn(ctx, "Authorization result has bad data format", map[string]interface{}{
					"auth_id": authID,
				})
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						tflog.Warn(ctx, "Authorization status field missing or not a string", map[string]interface{}{
							"auth_id": authID,
						})
						return true
					} else {
						statusLower := strings.ToLower(status)
						tflog.Debug(ctx, "Authorization status check", map[string]interface{}{
							"auth_id":     authID,
							"status":      status,
							"is_creating": statusLower == "creating",
						})
						// Will return false, when state is among final states like "created" or "failed"
						return statusLower == "creating"
					}
				}
				tflog.Warn(ctx, "Authorization result is nil", map[string]interface{}{
					"auth_id": authID,
				})
				return true
			}
		}))
	if err != nil {
		tflog.Error(ctx, "Failed to read authorization after retries", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("could to read session %s %w", authID, err)
	}
	finalStatus := strings.ToLower(resp["Status"].(string))
	if finalStatus != "created" {
		tflog.Error(ctx, "Authorization not in created status", map[string]interface{}{
			"auth_id": authID,
			"status":  resp["Status"],
		})
		return nil, fmt.Errorf("error read session %s", authID)
	}
	tflog.Debug(ctx, "Authorization successfully created", map[string]interface{}{
		"auth_id": authID,
	})
	return resp, nil

}

// Authorizations
func (c *Client) CreateAuthorization(ctx context.Context, aName, description, permissions, resourceName string) (*CreateAuthorizationResponse, error) {
	tflog.Debug(ctx, "CreateAuthorization called", map[string]interface{}{
		"name":        aName,
		"resource":    resourceName,
		"permissions": permissions,
	})
	req := CreateAuthorizationRequest{
		AuthorizationName: aName,
		Resource:          resourceName,
		Description:       description,
		Permissions:       permissions,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for creating authorization", map[string]interface{}{
			"name":  aName,
			"error": err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.authorizationAPI()), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for creating authorization", map[string]interface{}{
			"name":  aName,
			"error": err.Error(),
		})
		return nil, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for creating authorization", map[string]interface{}{
			"name":  aName,
			"error": err.Error(),
		})
		return nil, err
	}
	if _response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate authorization detected", map[string]interface{}{
			"name": req.AuthorizationName,
		})
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			tflog.Error(ctx, "Failed to decode error response for authorization", map[string]interface{}{
				"name":         req.AuthorizationName,
				"decode_error": err.Error(),
			})
		}
		tflog.Error(ctx, "Failed to create authorization", map[string]interface{}{
			"name":        req.AuthorizationName,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response CreateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		tflog.Error(ctx, "Failed to decode success response body for authorization", map[string]interface{}{
			"name":  req.AuthorizationName,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Authorization created, waiting for it to be functional", map[string]interface{}{
		"id": response.ID,
	})
	// now wait for it to be functional
	if _, err := c.ReadAuthorization(ctx, response.ID, true); err != nil {
		tflog.Error(ctx, "Failed to verify authorization after creation", map[string]interface{}{
			"id":    response.ID,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to create authorization %s", response.ID)
	}
	tflog.Debug(ctx, "Authorization successfully created and verified", map[string]interface{}{
		"id":   response.ID,
		"name": aName,
	})
	return &response, nil
}

func (c *Client) UpdateAuthorization(ctx context.Context, authID, newName, newDescription, permission, resourceType string) (*UpdateAuthorizationResponse, error) {
	tflog.Debug(ctx, "UpdateAuthorization called", map[string]interface{}{
		"auth_id":       authID,
		"new_name":      newName,
		"resource_type": resourceType,
	})
	req := UpdateAuthorizationRequest{
		AuthorizationName:        newName,
		AuthorizationDescription: newDescription,
		Permissions:              permission,
		ResourceType:             resourceType,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for updating authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.authorizationAPI(), authID), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for updating authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for updating authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, err
	}
	if _response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate authorization name during update", map[string]interface{}{
			"auth_id": authID,
			"name":    req.AuthorizationName,
		})
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			tflog.Error(ctx, "Failed to decode error response for authorization update", map[string]interface{}{
				"auth_id":      authID,
				"decode_error": err.Error(),
			})
		}
		tflog.Error(ctx, "Failed to update authorization", map[string]interface{}{
			"auth_id":     authID,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response UpdateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		tflog.Error(ctx, "Failed to decode success response body for authorization update", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Authorization successfully updated", map[string]interface{}{
		"auth_id": authID,
	})
	return &response, nil
}

func (c *Client) DeleteAuthorization(ctx context.Context, authID string) (bool, error) {

	tflog.Debug(ctx, "DeleteAuthorization called", map[string]interface{}{
		"auth_id": authID,
	})

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for deleting authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for deleting authorization", map[string]interface{}{
			"auth_id": authID,
			"error":   err.Error(),
		})
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {

		d, _ := io.ReadAll(response.Body) // read body to completion to allow connection reuse
		tflog.Error(ctx, "Failed to delete authorization", map[string]interface{}{
			"auth_id":       authID,
			"status_code":   response.StatusCode,
			"response_body": string(d),
		})

		return false, fmt.Errorf("error deleting authorization %s", authID)
	}
	tflog.Debug(ctx, "Authorization successfully deleted", map[string]interface{}{
		"auth_id": authID,
	})
	return true, nil
}

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
		Timeout:           pauseTimeout,
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
		return nil, fmt.Errorf("failed to create session %s", response.ID)
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
		Timeout:           pauseTimeout,
		Memory:            memory,
		CPU:               cpu,
		UsersTags:         tags,
		Groups:            groups,
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

// Resources / Integrations
func (c *Client) CreateResource(
	ctx context.Context,
	name, rType string,
	yamlRConfig []byte,
	tags []string,
) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "CreateResource called", map[string]interface{}{
		"name": name,
		"type": rType,
	})
	req := CreateResourceRequest{
		IntegrationType: rType,
		Name:            name,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for creating resource", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.resourceAPI()), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for creating resource", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for creating resource", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return nil, err
	}
	if response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate resource detected", map[string]interface{}{
			"name": name,
		})
		return nil, fmt.Errorf("duplicate resource with name %s", name)
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to create resource", map[string]interface{}{
			"name":        req.Name,
			"status_code": response.StatusCode,
		})
		return nil, fmt.Errorf("error creating resource %s", req.Name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for resource", map[string]interface{}{
			"name":  req.Name,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Resource successfully created", map[string]interface{}{
		"name": name,
		"id":   resp.ID,
	})
	return &resp, nil
}

func (c *Client) UpdateResource(
	ctx context.Context,
	resourceID string,
	rType string,
	yamlRConfig []byte,
	tags []string,
) (*UpdateResourceResponse, error) {
	tflog.Debug(ctx, "UpdateResource called", map[string]interface{}{
		"resource_id": resourceID,
		"type":        rType,
	})
	req := UpdateResourceRequest{
		IntegrationType: rType,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		tflog.Error(ctx, "Failed to encode request body for updating resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.resourceAPI(), resourceID), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for updating resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for updating resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, err
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to update resource", map[string]interface{}{
			"resource_id": resourceID,
			"status_code": response.StatusCode,
		})
		return nil, fmt.Errorf("error updating resource %s", resourceID)
	}

	var updateResourceResponse UpdateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&updateResourceResponse); err != nil {
		tflog.Error(ctx, "Failed to decode response body for resource update", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, err
	}
	tflog.Debug(ctx, "Resource successfully updated", map[string]interface{}{
		"resource_id": resourceID,
	})
	return &updateResourceResponse, nil
}

func (c *Client) DeleteResource(ctx context.Context, resourceID, resourceName string) (bool, error) {
	tflog.Debug(ctx, "DeleteResource called", map[string]interface{}{
		"resource_id": resourceID,
		"name":        resourceName,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.resourceAPI(), resourceID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for deleting resource", map[string]interface{}{
			"name":  resourceName,
			"error": err.Error(),
		})
		return false, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for deleting resource", map[string]interface{}{
			"name":  resourceName,
			"error": err.Error(),
		})
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			tflog.Error(ctx, "Failed to decode error response for resource deletion", map[string]interface{}{
				"name":         resourceName,
				"decode_error": decodeErr.Error(),
			})
		}
		tflog.Error(ctx, "Failed to delete resource", map[string]interface{}{
			"name":        resourceName,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		msg := fmt.Sprintf("error deleting resource %s", resourceName)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	tflog.Debug(ctx, "Resource successfully deleted", map[string]interface{}{
		"name": resourceName,
	})
	return true, nil
}

func (c *Client) CreateScript(ctx context.Context, name, command, endpoint string) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "CreateScript called", map[string]interface{}{
		"name":     name,
		"endpoint": endpoint,
	})
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		tflog.Error(ctx, "Failed to encode request body for creating script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.scriptAPI()), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for creating script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for creating script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return nil, err
	}
	if response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate script detected", map[string]interface{}{
			"name": name,
		})
		return nil, fmt.Errorf("duplicate script with name %s", name)
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to create script", map[string]interface{}{
			"name":        name,
			"status_code": response.StatusCode,
		})
		return nil, fmt.Errorf("error creating script %s", name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Script successfully created", map[string]interface{}{
		"name": name,
		"id":   resp.ID,
	})
	return &resp, nil
}

func (c *Client) UpdateScript(ctx context.Context, id, name, command, endpoint *string) (any, error) {
	tflog.Debug(ctx, "UpdateScript called", map[string]interface{}{
		"script_id": *id,
		"name":      *name,
	})
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		tflog.Error(ctx, "Failed to encode request body for updating script", map[string]interface{}{
			"script_id": *id,
			"error":     err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.scriptAPI(), *id), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for updating script", map[string]interface{}{
			"script_id": *id,
			"error":     err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for updating script", map[string]interface{}{
			"script_id": *id,
			"error":     err.Error(),
		})
		return nil, err
	}
	if response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate script name during update", map[string]interface{}{
			"script_id": *id,
			"name":      *name,
		})
		return nil, fmt.Errorf("duplicate script with name %s", *name)
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to update script", map[string]interface{}{
			"script_id":   *id,
			"status_code": response.StatusCode,
		})
		return nil, fmt.Errorf("error updating script %s", *name)
	}
	tflog.Debug(ctx, "Script successfully updated", map[string]interface{}{
		"script_id": *id,
	})
	return nil, nil
}

func (c *Client) DeleteScript(ctx context.Context, id, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteScript called", map[string]interface{}{
		"script_id": id,
		"name":      name,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.scriptAPI(), id), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for deleting script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return false, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for deleting script", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			tflog.Error(ctx, "Failed to decode error response for script deletion", map[string]interface{}{
				"name":         name,
				"decode_error": decodeErr.Error(),
			})
		}
		tflog.Error(ctx, "Failed to delete script", map[string]interface{}{
			"name":        name,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		msg := fmt.Sprintf("error deleting script %s", name)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	tflog.Debug(ctx, "Script successfully deleted", map[string]interface{}{
		"name": name,
	})
	return true, nil
}

func (c *Client) CreateTeam(ctx context.Context, name *string, members, endpoints *[]string) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "CreateTeam called", map[string]interface{}{
		"name": *name,
	})
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		tflog.Error(ctx, "Failed to encode request body for creating team", map[string]interface{}{
			"name":  *name,
			"error": err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.teamAPI()), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for creating team", map[string]interface{}{
			"name":  *name,
			"error": err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for creating team", map[string]interface{}{
			"name":  *name,
			"error": err.Error(),
		})
		return nil, err
	}
	if response.StatusCode == 409 {
		tflog.Error(ctx, "Duplicate group/team detected", map[string]interface{}{
			"name": *name,
		})
		return nil, fmt.Errorf("duplicate group with name %s", *name)
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(ctx, response)
		if err != nil {
			tflog.Error(ctx, "Failed to decode error response for team creation", map[string]interface{}{
				"name":  *name,
				"error": err.Error(),
			})
			return nil, fmt.Errorf("error creating group %s", *name)
		}
		tflog.Error(ctx, "Failed to create team", map[string]interface{}{
			"name":    *name,
			"message": decodedMsg,
		})
		return nil, errors.New(decodedMsg)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for team", map[string]interface{}{
			"name":  *name,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Team successfully created", map[string]interface{}{
		"name": *name,
		"id":   resp.ID,
	})
	return &resp, nil
}

func (c *Client) GetTeam(ctx context.Context, id string) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "GetTeam called", map[string]interface{}{
		"team_id": id,
	})
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.teamAPI(), id), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for getting team", map[string]interface{}{
			"team_id": id,
			"error":   err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for getting team", map[string]interface{}{
			"team_id": id,
			"error":   err.Error(),
		})
		return nil, err
	}
	if response.StatusCode != 200 {
		tflog.Error(ctx, "Failed to get team", map[string]interface{}{
			"team_id":     id,
			"status_code": response.StatusCode,
		})
		return nil, fmt.Errorf("error getting group %s", id)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for team", map[string]interface{}{
			"team_id": id,
			"error":   err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Team successfully retrieved", map[string]interface{}{
		"team_id": id,
	})
	return &resp, nil
}

func (c *Client) UpdateTeam(ctx context.Context, id, name *string, members, endpoints *[]string) (any, error) {
	tflog.Debug(ctx, "UpdateTeam called", map[string]interface{}{
		"team_id": *id,
		"name":    *name,
	})
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		tflog.Error(ctx, "Failed to encode request body for updating team", map[string]interface{}{
			"team_id": *id,
			"error":   err.Error(),
		})
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.teamAPI(), *id), payloadBuf)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for updating team", map[string]interface{}{
			"team_id": *id,
			"error":   err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for updating team", map[string]interface{}{
			"team_id": *id,
			"error":   err.Error(),
		})
		return nil, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(ctx, response)
		if err != nil {
			tflog.Error(ctx, "Failed to decode error response for team update", map[string]interface{}{
				"team_id": *id,
				"error":   err.Error(),
			})
			return nil, fmt.Errorf("error updating group %s", *name)
		}
		tflog.Error(ctx, "Failed to update team", map[string]interface{}{
			"team_id": *id,
			"message": decodedMsg,
		})
		return nil, errors.New(decodedMsg)
	}
	tflog.Debug(ctx, "Team successfully updated", map[string]interface{}{
		"team_id": *id,
	})
	return nil, nil
}

func (c *Client) DeleteTeam(ctx context.Context, id, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteTeam called", map[string]interface{}{
		"team_id": id,
		"name":    name,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.teamAPI(), id), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for deleting team", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to make request to adaptive API for deleting team", map[string]interface{}{
			"name":  name,
			"error": err.Error(),
		})
		return false, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(ctx, response)
		if err != nil {
			tflog.Error(ctx, "Failed to decode error response for team deletion", map[string]interface{}{
				"name":  name,
				"error": err.Error(),
			})
			return false, fmt.Errorf("error deleting group %s", name)
		}
		tflog.Error(ctx, "Failed to delete team", map[string]interface{}{
			"name":    name,
			"message": decodedMsg,
		})
		return false, errors.New(decodedMsg)
	}
	tflog.Debug(ctx, "Team successfully deleted", map[string]interface{}{
		"name": name,
	})
	return true, nil
}

func decodeError(ctx context.Context, response *http.Response) (reason string, decodeErr error) {
	var errReason ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errReason); err != nil {
		tflog.Error(ctx, "Failed to decode error response", map[string]interface{}{
			"status_code": response.StatusCode,
			"error":       err.Error(),
		})
		return "", fmt.Errorf("failed to decode response body. err %w", err)
	} else {
		reason = errReason.Error
		tflog.Debug(ctx, "Decoded error response", map[string]interface{}{
			"reason": reason,
		})
		return
	}
}
