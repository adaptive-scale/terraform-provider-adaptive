package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
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

func (c *Client) do(req *http.Request) (*http.Response, error) {
	log.Printf("[DEBUG] Making HTTP request: method=%s, url=%s", req.Method, req.URL.String())
	req.Header.Set("Authorization", c.serviceToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to make HTTP request: method=%s, url=%s, error=%v", req.Method, req.URL.String(), err)
		return nil, err
	}

	log.Printf("[DEBUG] HTTP response received: status_code=%d, url=%s", res.StatusCode, req.URL.String())
	if res.StatusCode == 401 {
		log.Printf("[ERROR] Authentication failed: bad token, url=%s", req.URL.String())
		return nil, errors.New("bad token. please check your service token")
	}
	return res, err
}

func _readAuthorization(c *Client, authID string) (map[string]interface{}, error) {
	log.Printf("[DEBUG] Reading authorization: auth_id=%s", authID)
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for reading authorization: auth_id=%s, error=%v", authID, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to request adaptive api for authorization: auth_id=%s, error=%v", authID, err)
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		log.Printf("[ERROR] Unexpected status code reading authorization: auth_id=%s, status_code=%d, expected=%d", authID, response.StatusCode, http.StatusAccepted)
		return nil, fmt.Errorf("error read authorization %s", authID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for authorization: auth_id=%s, error=%v", authID, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Successfully read authorization: auth_id=%s, response=%+v", authID, resp)
	return resp, nil
}

func (c *Client) ReadAuthorization(authID string, waitForStatus bool) (any, error) {
	log.Printf("[DEBUG] ReadAuthorization called: auth_id=%s, wait_for_status=%v", authID, waitForStatus)
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 20
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readAuthorization(c, authID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				log.Printf("[WARN] Authorization result has bad data format: auth_id=%s", authID)
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						log.Printf("[WARN] Authorization status field missing or not a string: auth_id=%s", authID)
						return true
					} else {
						statusLower := strings.ToLower(status)
						log.Printf("[DEBUG] Authorization status check: auth_id=%s, status=%s, is_creating=%v", authID, status, statusLower == "creating")
						// Will return false, when state is among final states like "created" or "failed"
						return statusLower == "creating"
					}
				}
				log.Printf("[WARN] Authorization result is nil: auth_id=%s", authID)
				return true
			}
		}))
	if err != nil {
		log.Printf("[ERROR] Failed to read authorization after retries: auth_id=%s, error=%v", authID, err)
		return nil, fmt.Errorf("could to read session %s %w", authID, err)
	}
	finalStatus := strings.ToLower(resp["Status"].(string))
	if finalStatus != "created" {
		log.Printf("[ERROR] Authorization not in created status: auth_id=%s, status=%s", authID, resp["Status"])
		return nil, fmt.Errorf("error read session %s", authID)
	}
	log.Printf("[DEBUG] Authorization successfully created: auth_id=%s", authID)
	return resp, nil

}

// Authorizations
func (c *Client) CreateAuthorization(ctx context.Context, aName, description, permissions, resourceName string) (*CreateAuthorizationResponse, error) {
	log.Printf("[DEBUG] CreateAuthorization called: name=%s, resource=%s, permissions=%s", aName, resourceName, permissions)
	req := CreateAuthorizationRequest{
		AuthorizationName: aName,
		Resource:          resourceName,
		Description:       description,
		Permissions:       permissions,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		log.Printf("[ERROR] Failed to encode request body for creating authorization: name=%s, error=%v", aName, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.authorizationAPI()), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for creating authorization: name=%s, error=%v", aName, err)
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for creating authorization: name=%s, error=%v", aName, err)
		return nil, err
	}
	if _response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate authorization detected: name=%s", req.AuthorizationName)
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			log.Printf("[ERROR] Failed to decode error response for authorization: name=%s, decode_error=%v", req.AuthorizationName, err)
		}
		log.Printf("[ERROR] Failed to create authorization: name=%s, status_code=%d, reason=%s", req.AuthorizationName, _response.StatusCode, errReason)
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response CreateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		log.Printf("[ERROR] Failed to decode success response body for authorization: name=%s, error=%v", req.AuthorizationName, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Authorization created, waiting for it to be functional: id=%s", response.ID)
	// now wait for it to be functional
	if _, err := c.ReadAuthorization(response.ID, true); err != nil {
		log.Printf("[ERROR] Failed to verify authorization after creation: id=%s, error=%v", response.ID, err)
		return nil, fmt.Errorf("failed to create authorization %s", response.ID)
	}
	log.Printf("[DEBUG] Authorization successfully created and verified: id=%s, name=%s", response.ID, aName)
	return &response, nil
}

func (c *Client) UpdateAuthorization(ctx context.Context, authID, newName, newDescription, permission, resourceType string) (*UpdateAuthorizationResponse, error) {
	log.Printf("[DEBUG] UpdateAuthorization called: auth_id=%s, new_name=%s, resource_type=%s", authID, newName, resourceType)
	req := UpdateAuthorizationRequest{
		AuthorizationName:        newName,
		AuthorizationDescription: newDescription,
		Permissions:              permission,
		ResourceType:             resourceType,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		log.Printf("[ERROR] Failed to encode request body for updating authorization: auth_id=%s, error=%v", authID, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.authorizationAPI(), authID), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for updating authorization: auth_id=%s, error=%v", authID, err)
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for updating authorization: auth_id=%s, error=%v", authID, err)
		return nil, err
	}
	if _response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate authorization name during update: auth_id=%s, name=%s", authID, req.AuthorizationName)
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			log.Printf("[ERROR] Failed to decode error response for authorization update: auth_id=%s, decode_error=%v", authID, err)
		}
		log.Printf("[ERROR] Failed to update authorization: auth_id=%s, status_code=%d, reason=%s", authID, _response.StatusCode, errReason)
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response UpdateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		log.Printf("[ERROR] Failed to decode success response body for authorization update: auth_id=%s, error=%v", authID, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Authorization successfully updated: auth_id=%s", authID)
	return &response, nil
}

func (c *Client) DeleteAuthorization(ctx context.Context, authID string) (bool, error) {

	log.Printf("[DEBUG] DeleteAuthorization called: auth_id=%s", authID)

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for deleting authorization: auth_id=%s, error=%v", authID, err)
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for deleting authorization: auth_id=%s, error=%v", authID, err)
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {

		d, _ := io.ReadAll(response.Body) // read body to completion to allow connection reuse
		log.Printf("[ERROR] Failed to delete authorization: auth_id=%s, status_code=%d, response_body=%s", authID, response.StatusCode, string(d))

		return false, fmt.Errorf("error deleting authorization %s", authID)
	}
	log.Printf("[DEBUG] Authorization successfully deleted: auth_id=%s", authID)
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
	log.Printf("[DEBUG] CreateSession called: name=%s, resource=%s, authorization=%s, cluster=%s, type=%s", sessionName, resourceName, authorizationName, clusterName, sessionType)
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
		log.Printf("[ERROR] Failed to encode request body for creating session: name=%s, error=%v", sessionName, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.sessionAPI()), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for creating session: name=%s, error=%v", sessionName, err)
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for creating session: name=%s, error=%v", sessionName, err)
		return nil, err
	}
	if _response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate session detected: name=%s", sessionName)
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}

	if _response.StatusCode != 200 {
		errReason, err := io.ReadAll(_response.Body)
		if err != nil {
			log.Printf("[ERROR] Failed to read error response body for session: name=%s, read_error=%v", req.SessionName, err)
			return nil, fmt.Errorf("error decoding response %s", err)
		}
		log.Printf("[ERROR] Failed to create session: name=%s, status_code=%d, reason=%s", req.SessionName, _response.StatusCode, string(errReason))
		return nil, fmt.Errorf("error creating session %s, reason %s", req.SessionName, string(errReason))
	}

	var response CreateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		log.Printf("[ERROR] Failed to decode success response body for session: name=%s, error=%v", req.SessionName, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Session created, waiting for it to be functional: id=%s", response.ID)
	// now wait for it to be functional
	if _, err := c.ReadSession(response.ID, true); err != nil {
		log.Printf("[ERROR] Failed to verify session after creation: id=%s, error=%v", response.ID, err)
		return nil, fmt.Errorf("failed to create session %s", response.ID)
	}
	log.Printf("[DEBUG] Session successfully created and verified: id=%s, name=%s", response.ID, sessionName)
	return &response, nil
}

func _readSession(c *Client, sessionID string) (map[string]interface{}, error) {
	log.Printf("[DEBUG] Reading session: session_id=%s", sessionID)
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for reading session: session_id=%s, error=%v", sessionID, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to request adaptive api for session: session_id=%s, error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		log.Printf("[ERROR] Unexpected status code reading session: session_id=%s, status_code=%d, expected=%d", sessionID, response.StatusCode, http.StatusAccepted)
		return nil, fmt.Errorf("error read session %s", sessionID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for session: session_id=%s, error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Successfully read session: session_id=%s, response=%+v", sessionID, resp)
	return resp, nil
}

/*
waitForStatus: if true, will wait for session to be active/fail before returning
*/
func (c *Client) ReadSession(sessionID string, waitForStatus bool) (map[string]interface{}, error) {
	log.Printf("[DEBUG] ReadSession called: session_id=%s, wait_for_status=%v", sessionID, waitForStatus)
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 30
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			log.Printf("[DEBUG] Session status check: %v", intermedResult)
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				log.Printf("[WARN] Session result has bad data format: session_id=%s", sessionID)
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						log.Printf("[WARN] Session status field missing or not a string: session_id=%s", sessionID)
						return true
					} else {
						statusLower := strings.ToLower(status)
						log.Printf("[DEBUG] Session status check: session_id=%s, status=%s, is_creating=%v", sessionID, status, statusLower == "creating")
						// Will return false, when state is among final states like "created" or "failed"
						return statusLower == "creating"
					}
				}
				log.Printf("[WARN] Session result is nil: session_id=%s", sessionID)
				return true
			}
		}))
	if err != nil {
		log.Printf("[ERROR] Failed to read session after retries: session_id=%s, error=%v", sessionID, err)
		return nil, fmt.Errorf("could to create session %s", sessionID)
	}
	finalStatus := strings.ToLower(resp["Status"].(string))
	if finalStatus != "created" {
		log.Printf("[ERROR] Session not in created status: session_id=%s, status=%s", sessionID, resp["Status"])
		return nil, fmt.Errorf("error create session %s", sessionID)
	}
	log.Printf("[DEBUG] Session successfully created: session_id=%s", sessionID)
	return resp, nil
}

func (c *Client) UpdateSession(
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
	log.Printf("[DEBUG] UpdateSession called: session_id=%s, name=%s, resource=%s", sessionID, sessionName, resourceName)
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
		log.Printf("[ERROR] Failed to encode request body for updating session: session_id=%s, error=%v", sessionID, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.sessionAPI(), sessionID), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for updating session: session_id=%s, error=%v", sessionID, err)
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for updating session: session_id=%s, error=%v", sessionID, err)
		return nil, err
	}
	if _response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate session name during update: session_id=%s, name=%s", sessionID, sessionName)
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			log.Printf("[ERROR] Failed to decode error response for session update: session_id=%s, decode_error=%v", sessionID, decodeErr)
		}
		log.Printf("[ERROR] Failed to update session: session_id=%s, status_code=%d, reason=%s", sessionID, _response.StatusCode, errReason)
		return nil, fmt.Errorf("error updating session %s, reason %s", req.SessionName, errReason)
	}
	var response UpdateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		log.Printf("[ERROR] Failed to decode success response body for session update: session_id=%s, error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Session successfully updated: session_id=%s", sessionID)
	return &response, nil
}

func (c *Client) DeleteSession(sessionID string) (bool, error) {
	log.Printf("[DEBUG] DeleteSession called: session_id=%s", sessionID)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for deleting session: session_id=%s, error=%v", sessionID, err)
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for deleting session: session_id=%s, error=%v", sessionID, err)
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to initiate session deletion: session_id=%s, status_code=%d", sessionID, response.StatusCode)
		return false, fmt.Errorf("error deleting session %s", sessionID)
	}
	log.Printf("[DEBUG] Delete request successful, monitoring session termination: session_id=%s", sessionID)
	// Once delete request is succesful, we check for status of session
	timeout := time.Second * 10
	retryForStatus := 20

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				log.Printf("[WARN] Session deletion check has bad data format: session_id=%s", sessionID)
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						log.Printf("[WARN] Session status field missing during deletion: session_id=%s", sessionID)
						return false
					} else {
						statusLower := strings.ToLower(status)
						log.Printf("[DEBUG] Session deletion status check: session_id=%s, status=%s", sessionID, status)

						// Return true to keep retrying if NOT in a terminal state
						terminatedOrFailed := (statusLower == "terminated" || statusLower == "marked-for-deletion" || statusLower == "failed" || statusLower == "failed-to-restart")
						if terminatedOrFailed {
							log.Printf("[DEBUG] Session in terminal state, forcing deletion: session_id=%s, status=%s", sessionID, status)
							_, err2 := c.deleteSession(sessionID)
							if err2 != nil {
								log.Printf("[ERROR] Failed to force delete session: session_id=%s, error=%v", sessionID, err2)
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
		log.Printf("[ERROR] Failed to read session during deletion: session_id=%s, error=%v", sessionID, err)
		return false, fmt.Errorf("could to read session %s %w", sessionID, err)
	}

	if status, ok := resp["Status"].(string); ok {
		statusLower := strings.ToLower(status)
		log.Printf("[DEBUG] Final session deletion status: session_id=%s, status=%s", sessionID, status)
		if statusLower == "terminated" || statusLower == "marked-for-deletion" || statusLower == "does-not-exist" {
			log.Printf("[DEBUG] Session successfully deleted: session_id=%s", sessionID)
			return true, nil
		}
		log.Printf("[ERROR] Unexpected session status after deletion: session_id=%s, status=%s", sessionID, statusLower)
		return false, fmt.Errorf("error read session %s", sessionID)
	} else {
		log.Printf("[ERROR] Could not determine status for session: session_id=%s", sessionID)
		return false, errors.New("could not delete session")
	}

	return true, nil
}

func (c *Client) deleteSession(sessionID string) (bool, error) {
	log.Printf("[DEBUG] Force deleting session: session_id=%s", sessionID)

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/forcedelete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for force deleting session: session_id=%s, error=%v", sessionID, err)
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for force deleting session: session_id=%s, error=%v", sessionID, err)
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}

	if _response.StatusCode != 200 {
		log.Printf("[ERROR] Force delete failed: session_id=%s, status_code=%d", sessionID, _response.StatusCode)
		return false, fmt.Errorf("error force deleting session %s", sessionID)
	}

	log.Printf("[DEBUG] Session force deleted successfully: session_id=%s", sessionID)
	return true, nil
}

// Resources / Integrations
func (c *Client) CreateResource(
	ctx context.Context,
	name, rType string,
	yamlRConfig []byte,
	tags []string,
) (*CreateResourceResponse, error) {
	log.Printf("[DEBUG] CreateResource called: name=%s, type=%s", name, rType)
	req := CreateResourceRequest{
		IntegrationType: rType,
		Name:            name,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		log.Printf("[ERROR] Failed to encode request body for creating resource: name=%s, error=%v", name, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.resourceAPI()), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for creating resource: name=%s, error=%v", name, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for creating resource: name=%s, error=%v", name, err)
		return nil, err
	}
	if response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate resource detected: name=%s", name)
		return nil, fmt.Errorf("duplicate resource with name %s", name)
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to create resource: name=%s, status_code=%d", req.Name, response.StatusCode)
		return nil, fmt.Errorf("error creating resource %s", req.Name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for resource: name=%s, error=%v", req.Name, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Resource successfully created: name=%s, id=%s", name, resp.ID)
	return &resp, nil
}

func (c *Client) UpdateResource(
	resourceID string,
	rType string,
	yamlRConfig []byte,
	tags []string,
) (*UpdateResourceResponse, error) {
	log.Printf("[DEBUG] UpdateResource called: resource_id=%s, type=%s", resourceID, rType)
	req := UpdateResourceRequest{
		IntegrationType: rType,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		log.Printf("[ERROR] Failed to encode request body for updating resource: resource_id=%s, error=%v", resourceID, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.resourceAPI(), resourceID), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for updating resource: resource_id=%s, error=%v", resourceID, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for updating resource: resource_id=%s, error=%v", resourceID, err)
		return nil, err
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to update resource: resource_id=%s, status_code=%d", resourceID, response.StatusCode)
		return nil, fmt.Errorf("error updating resource %s", resourceID)
	}

	var updateResourceResponse UpdateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&updateResourceResponse); err != nil {
		log.Printf("[ERROR] Failed to decode response body for resource update: resource_id=%s, error=%v", resourceID, err)
		return nil, err
	}
	log.Printf("[DEBUG] Resource successfully updated: resource_id=%s", resourceID)
	return &updateResourceResponse, nil
}

func (c *Client) DeleteResource(resourceID, resourceName string) (bool, error) {
	log.Printf("[DEBUG] DeleteResource called: resource_id=%s, name=%s", resourceID, resourceName)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.resourceAPI(), resourceID), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for deleting resource: name=%s, error=%v", resourceName, err)
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for deleting resource: name=%s, error=%v", resourceName, err)
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			log.Printf("[ERROR] Failed to decode error response for resource deletion: name=%s, decode_error=%v", resourceName, decodeErr)
		}
		log.Printf("[ERROR] Failed to delete resource: name=%s, status_code=%d, reason=%s", resourceName, _response.StatusCode, errReason)
		msg := fmt.Sprintf("error deleting resource %s", resourceName)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	log.Printf("[DEBUG] Resource successfully deleted: name=%s", resourceName)
	return true, nil
}

func (c *Client) CreateScript(ctx context.Context, name, command, endpoint string) (*CreateResourceResponse, error) {
	log.Printf("[DEBUG] CreateScript called: name=%s, endpoint=%s", name, endpoint)
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		log.Printf("[ERROR] Failed to encode request body for creating script: name=%s, error=%v", name, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.scriptAPI()), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for creating script: name=%s, error=%v", name, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for creating script: name=%s, error=%v", name, err)
		return nil, err
	}
	if response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate script detected: name=%s", name)
		return nil, fmt.Errorf("duplicate script with name %s", name)
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to create script: name=%s, status_code=%d", name, response.StatusCode)
		return nil, fmt.Errorf("error creating script %s", name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for script: name=%s, error=%v", name, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Script successfully created: name=%s, id=%s", name, resp.ID)
	return &resp, nil
}

func (c *Client) UpdateScript(ctx context.Context, id, name, command, endpoint *string) (any, error) {
	log.Printf("[DEBUG] UpdateScript called: script_id=%s, name=%s", *id, *name)
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		log.Printf("[ERROR] Failed to encode request body for updating script: script_id=%s, error=%v", *id, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.scriptAPI(), *id), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for updating script: script_id=%s, error=%v", *id, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for updating script: script_id=%s, error=%v", *id, err)
		return nil, err
	}
	if response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate script name during update: script_id=%s, name=%s", *id, *name)
		return nil, fmt.Errorf("duplicate script with name %s", *name)
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to update script: script_id=%s, status_code=%d", *id, response.StatusCode)
		return nil, fmt.Errorf("error updating script %s", *name)
	}
	log.Printf("[DEBUG] Script successfully updated: script_id=%s", *id)
	return nil, nil
}

func (c *Client) DeleteScript(ctx context.Context, id, name string) (bool, error) {
	log.Printf("[DEBUG] DeleteScript called: script_id=%s, name=%s", id, name)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.scriptAPI(), id), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for deleting script: name=%s, error=%v", name, err)
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for deleting script: name=%s, error=%v", name, err)
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		decodeErr := json.NewDecoder(_response.Body).Decode(&errReason)
		if decodeErr != nil {
			log.Printf("[ERROR] Failed to decode error response for script deletion: name=%s, decode_error=%v", name, decodeErr)
		}
		log.Printf("[ERROR] Failed to delete script: name=%s, status_code=%d, reason=%s", name, _response.StatusCode, errReason)
		msg := fmt.Sprintf("error deleting script %s", name)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	log.Printf("[DEBUG] Script successfully deleted: name=%s", name)
	return true, nil
}

func (c *Client) CreateTeam(ctx context.Context, name *string, members, endpoints *[]string) (*CreateResourceResponse, error) {
	log.Printf("[DEBUG] CreateTeam called: name=%s", *name)
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		log.Printf("[ERROR] Failed to encode request body for creating team: name=%s, error=%v", *name, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.teamAPI()), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for creating team: name=%s, error=%v", *name, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for creating team: name=%s, error=%v", *name, err)
		return nil, err
	}
	if response.StatusCode == 409 {
		log.Printf("[ERROR] Duplicate group/team detected: name=%s", *name)
		return nil, fmt.Errorf("duplicate group with name %s", *name)
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			log.Printf("[ERROR] Failed to decode error response for team creation: name=%s, error=%v", *name, err)
			return nil, fmt.Errorf("error creating group %s", *name)
		}
		log.Printf("[ERROR] Failed to create team: name=%s, message=%s", *name, decodedMsg)
		return nil, errors.New(decodedMsg)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for team: name=%s, error=%v", *name, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Team successfully created: name=%s, id=%s", *name, resp.ID)
	return &resp, nil
}

func (c *Client) GetTeam(ctx context.Context, id string) (*CreateResourceResponse, error) {
	log.Printf("[DEBUG] GetTeam called: team_id=%s", id)
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.teamAPI(), id), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for getting team: team_id=%s, error=%v", id, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for getting team: team_id=%s, error=%v", id, err)
		return nil, err
	}
	if response.StatusCode != 200 {
		log.Printf("[ERROR] Failed to get team: team_id=%s, status_code=%d", id, response.StatusCode)
		return nil, fmt.Errorf("error getting group %s", id)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		log.Printf("[ERROR] Failed to decode response body for team: team_id=%s, error=%v", id, err)
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	log.Printf("[DEBUG] Team successfully retrieved: team_id=%s", id)
	return &resp, nil
}

func (c *Client) UpdateTeam(ctx context.Context, id, name *string, members, endpoints *[]string) (any, error) {
	log.Printf("[DEBUG] UpdateTeam called: team_id=%s, name=%s", *id, *name)
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		log.Printf("[ERROR] Failed to encode request body for updating team: team_id=%s, error=%v", *id, err)
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.teamAPI(), *id), payloadBuf)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for updating team: team_id=%s, error=%v", *id, err)
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for updating team: team_id=%s, error=%v", *id, err)
		return nil, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			log.Printf("[ERROR] Failed to decode error response for team update: team_id=%s, error=%v", *id, err)
			return nil, fmt.Errorf("error updating group %s", *name)
		}
		log.Printf("[ERROR] Failed to update team: team_id=%s, message=%s", *id, decodedMsg)
		return nil, errors.New(decodedMsg)
	}
	log.Printf("[DEBUG] Team successfully updated: team_id=%s", *id)
	return nil, nil
}

func (c *Client) DeleteTeam(ctx context.Context, id, name string) (bool, error) {
	log.Printf("[DEBUG] DeleteTeam called: team_id=%s, name=%s", id, name)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.teamAPI(), id), nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create HTTP request for deleting team: name=%s, error=%v", name, err)
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		log.Printf("[ERROR] Failed to make request to adaptive API for deleting team: name=%s, error=%v", name, err)
		return false, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			log.Printf("[ERROR] Failed to decode error response for team deletion: name=%s, error=%v", name, err)
			return false, fmt.Errorf("error deleting group %s", name)
		}
		log.Printf("[ERROR] Failed to delete team: name=%s, message=%s", name, decodedMsg)
		return false, errors.New(decodedMsg)
	}
	log.Printf("[DEBUG] Team successfully deleted: name=%s", name)
	return true, nil
}

func decodeError(response *http.Response) (reason string, decodeErr error) {
	var errReason ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errReason); err != nil {
		log.Printf("[ERROR] Failed to decode error response: status_code=%d, error=%v", response.StatusCode, err)
		return "", fmt.Errorf("failed to decode response body. err %w", err)
	} else {
		reason = errReason.Error
		log.Printf("[DEBUG] Decoded error response: reason=%s", reason)
		return
	}
}
