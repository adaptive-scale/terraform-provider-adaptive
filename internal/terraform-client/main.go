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
	req.Header.Set("Authorization", c.serviceToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 401 {
		return nil, errors.New("bad token. please check your service token")
	}
	return res, err
}

func _readAuthorization(c *Client, authID string) (map[string]interface{}, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("error read authorization %s", authID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return resp, nil
}

func (c *Client) ReadAuthorization(authID string, waitForStatus bool) (any, error) {
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
				// bad data format
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						return true
					} else {
						// Will return false, when state is among final states like "created" or "failed"
						return strings.ToLower(status) == "creating"
					}
				}
				return true
			}
		}))
	if err != nil {
		return nil, fmt.Errorf("could to read session %s %w", authID, err)
	}
	if strings.ToLower(resp["Status"].(string)) != "created" {
		return nil, fmt.Errorf("error read session %s", authID)
	}
	return resp, nil

}

// Authorizations
func (c *Client) CreateAuthorization(ctx context.Context, aName, description, permissions, resourceName string) (*CreateAuthorizationResponse, error) {
	req := CreateAuthorizationRequest{
		AuthorizationName: aName,
		Resource:          resourceName,
		Description:       description,
		Permissions:       permissions,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.authorizationAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		// TODO: update error, failed to make request to adaptive
		return nil, err
	}
	if _response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			log.Printf("decode error: %s", err)
		}
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response CreateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	// now wait for it to be functional
	if _, err := c.ReadAuthorization(response.ID, true); err != nil {
		// debug error
		// TODO: update error message with reason, after health check are implemented in inventorize
		return nil, fmt.Errorf("failed to create authorization %s", response.ID)
	}
	return &response, nil
}

func (c *Client) UpdateAuthorization(ctx context.Context, authID, newName, newDescription, permission, resourceType string) (*UpdateAuthorizationResponse, error) {
	req := UpdateAuthorizationRequest{
		AuthorizationName:        newName,
		AuthorizationDescription: newDescription,
		Permissions:              permission,
		ResourceType:             resourceType,
	}
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.authorizationAPI(), authID), payloadBuf)
	if err != nil {
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if _response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			log.Printf("decode error: %s", err)
		}
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response UpdateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &response, nil
}

func (c *Client) DeleteAuthorization(ctx context.Context, authID string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.authorizationAPI(), authID), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {
		return false, fmt.Errorf("error deleting authorization %s", authID)
	}
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
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.sessionAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if _response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}

	if _response.StatusCode != 200 {
		errReason, err := io.ReadAll(_response.Body)
		if err != nil {
			return nil, fmt.Errorf("error decoding response %s", err)
		}
		return nil, fmt.Errorf("error creating session %s, reason %s", req.SessionName, errReason)
	}

	var response CreateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	// now wait for it to be functional
	if _, err := c.ReadSession(response.ID, true); err != nil {
		// debug error
		// TODO: update error message with reason, after health check are implemented in inventorize
		return nil, fmt.Errorf("failed to create session %s", response.ID)
	}
	return &response, nil
}

func _readSession(c *Client, sessionID string) (map[string]interface{}, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("error read session %s", sessionID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return resp, nil
}

/*
waitForStatus: if true, will wait for session to be active/fail before returning
*/
func (c *Client) ReadSession(sessionID string, waitForStatus bool) (map[string]interface{}, error) {
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 30
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			log.Printf("status: %v", intermedResult)
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				// bad data format
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						return true
					} else {
						// Will return false, when state is among final states like "created" or "failed"
						return strings.ToLower(status) == "creating"
					}
				}
				return true
			}
		}))
	if err != nil {
		return nil, fmt.Errorf("could to create session %s", sessionID)
	}
	if strings.ToLower(resp["Status"].(string)) != "created" {
		return nil, fmt.Errorf("error create session %s", sessionID)
	}
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
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.sessionAPI(), sessionID), payloadBuf)
	if err != nil {
		return nil, err
	}

	_response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if _response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate session with name %s", sessionName)
	}
	if _response.StatusCode != 200 {
		var errReason string
		_ = json.NewDecoder(_response.Body).Decode(&errReason)
		return nil, fmt.Errorf("error updating session %s, reason %s", req.SessionName, errReason)
	}
	var response UpdateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &response, nil
}

func (c *Client) DeleteSession(sessionID string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != 200 {
		return false, fmt.Errorf("error deleting session %s", sessionID)
	}
	// Once delete request is succesful, we check for status of session
	timeout := time.Second * 10
	retryForStatus := 20

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readSession(c, sessionID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				// bad data format
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						return false
					} else {
						statusLower := strings.ToLower(status)

						log.Printf("Session %s status: %s\n", sessionID, statusLower)
						// Return true to keep retrying if NOT in a terminal state
						terminated := (statusLower == "terminated" || statusLower == "marked-for-deletion")
						if terminated {
							log.Println("Session terminated, continuing to delete...")
							_, err2 := c.deleteSession(sessionID)
							if err2 != nil {
								log.Printf("error deleting session %s: %s", sessionID, err)
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
		return false, fmt.Errorf("could to read session %s %w", sessionID, err)
	}

	if status, ok := resp["Status"].(string); ok {
		statusLower := strings.ToLower(status)
		if statusLower == "terminated" || statusLower == "marked-for-deletion" || statusLower == "does-not-exist" {
			return true, nil
		}
		return false, fmt.Errorf("error read session %s", sessionID)
	} else {
		// TODO: Add tracing ID
		return false, errors.New("could not delete session")
	}

	return true, nil
}

func (c *Client) deleteSession(sessionID string) (bool, error) {

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/forcedelete/%s", c.sessionAPI(), sessionID), nil)
	if err != nil {
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}

	if _response.StatusCode != 200 {
		return false, fmt.Errorf("error force deleting session %s", sessionID)
	}

	return true, nil
}

// Resources / Integrations
func (c *Client) CreateResource(
	ctx context.Context,
	name, rType string,
	yamlRConfig []byte,
	tags []string,
) (*CreateResourceResponse, error) {
	req := CreateResourceRequest{
		IntegrationType: rType,
		Name:            name,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.resourceAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate resource with name %s", name)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error creating resource %s", req.Name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

func (c *Client) UpdateResource(
	resourceID string,
	rType string,
	yamlRConfig []byte,
	tags []string,
) (*UpdateResourceResponse, error) {
	req := UpdateResourceRequest{
		IntegrationType: rType,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
	}

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.resourceAPI(), resourceID), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error updating resource %s", resourceID)
	}

	var updateResourceResponse UpdateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&updateResourceResponse); err != nil {
		return nil, err
	}
	return &updateResourceResponse, nil
}

func (c *Client) DeleteResource(resourceID, resourceName string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.resourceAPI(), resourceID), nil)
	if err != nil {
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		_ = json.NewDecoder(_response.Body).Decode(&errReason)
		msg := fmt.Sprintf("error deleting resource %s", resourceName)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	return true, nil
}

func (c *Client) CreateScript(ctx context.Context, name, command, endpoint string) (*CreateResourceResponse, error) {
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.scriptAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate script with name %s", name)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error creating script %s", name)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

func (c *Client) UpdateScript(ctx context.Context, id, name, command, endpoint *string) (any, error) {
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":     name,
		"Command":  command,
		"Endpoint": endpoint,
	}); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.scriptAPI(), *id), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate script with name %s", *name)
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error updating script %s", *name)
	}
	return nil, nil
}

func (c *Client) DeleteScript(ctx context.Context, id, name string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.scriptAPI(), id), nil)
	if err != nil {
		return false, err
	}

	_response, err := c.do(request)
	if err != nil {
		return false, err
	}
	if _response.StatusCode != 200 {
		var errReason string
		_ = json.NewDecoder(_response.Body).Decode(&errReason)
		msg := fmt.Sprintf("error deleting script %s", name)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, errors.New(msg)
	}
	return true, nil
}

func (c *Client) CreateTeam(ctx context.Context, name *string, members, endpoints *[]string) (*CreateResourceResponse, error) {
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.teamAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == 409 {
		return nil, fmt.Errorf("duplicate group with name %s", *name)
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			return nil, fmt.Errorf("error creating group %s", *name)
		}
		return nil, errors.New(decodedMsg)
	}
	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

func (c *Client) GetTeam(ctx context.Context, id string) (*CreateResourceResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.teamAPI(), id), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error getting group %s", id)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

func (c *Client) UpdateTeam(ctx context.Context, id, name *string, members, endpoints *[]string) (any, error) {
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	}); err != nil {
		err = fmt.Errorf("failed to json encode request body. err %w", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.teamAPI(), *id), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			return nil, fmt.Errorf("error updating group %s", *name)
		}
		return nil, errors.New(decodedMsg)
	}
	return nil, nil
}

func (c *Client) DeleteTeam(ctx context.Context, id, name string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.teamAPI(), id), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		return false, err
	}
	if response.StatusCode != 200 {
		decodedMsg, err := decodeError(response)
		if err != nil {
			return false, fmt.Errorf("error deleting group %s", name)
		}
		return false, errors.New(decodedMsg)
	}
	return true, nil
}

func decodeError(response *http.Response) (reason string, decodeErr error) {
	var errReason ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errReason); err != nil {
		return "", fmt.Errorf("failed to decode response body. err %w", err)
	} else {
		reason = errReason.Error
		return
	}
}
