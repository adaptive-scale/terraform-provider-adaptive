package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	// baseURL = "https://app.adaptive.com"
	baseURL = "http://localhost:8080/api/v1"
)

type Client struct {
	serviceToken string
	workspaceURL string
	httpClient   *http.Client
}

func NewClient(serviceToken, workspaceURL string) *Client {

	if workspaceURL == "" {
		workspaceURL = baseURL
	}

	return &Client{
		serviceToken: serviceToken,
		workspaceURL: workspaceURL,
		httpClient:   &http.Client{},
	}
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
	if res.StatusCode == 401 {
		return nil, errors.New("bad token. please check your service token")
	}
	// if res.StatusCode == 409 {
	// 	isResourceAPI := strings.Contains(req.URL.String(), "/resource")
	// 	entity := "session"
	// 	if isResourceAPI {
	// 		entity = "resource"
	// 	}
	// 	return nil, fmt.Errorf("duplicate %s", entity)
	// }
	return res, err
}

// Sessions
func (c *Client) CreateSession(ctx context.Context, sessionName, resourceName, authorizationName, clusterName, ttl, sessionType string) (*CreateSessionResponse, error) {
	req := CreateSessionRequest{
		SessionName:       sessionName,
		ResourceName:      resourceName,
		ClusterName:       clusterName,
		AuthorizationName: authorizationName,
		SessionTTL:        ttl,
		SessionType:       sessionType,
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
	// if _response.StatusCode != 200 {
	// 	return nil, fmt.Errorf("error creating session %s", req.SessionName)
	// }
	if _response.StatusCode != 200 {
		var errReason string
		err := json.NewDecoder(_response.Body).Decode(&errReason)
		if err != nil {
			tflog.Trace(ctx, fmt.Sprintf("decode error: %s", err))
		}
		return nil, fmt.Errorf("error creating session %s, reason %s", req.SessionName, errReason)
	}

	var response CreateSessionResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &response, nil
}

func (c *Client) ReadSession(sessionID string) {
	panic("not implemented")
}

func (c *Client) UpdateSession(sessionID, sessionName, resourceName, authorizationName, clusterName, ttl, sessionType string) (*UpdateSessionResponse, error) {
	req := UpdateSessionRequest{
		SessionName:       sessionName,
		ResourceName:      resourceName,
		ClusterName:       clusterName,
		SessionType:       sessionType,
		AuthorizationName: authorizationName,
		SessionTTL:        ttl,
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
	return true, nil
}

// Resources / Integrations
func (c *Client) CreateResource(ctx context.Context, name, rType string, yamlRConfig []byte) (*CreateResourceResponse, error) {
	req := CreateResourceRequest{
		IntegrationType: rType,
		Name:            name,
		Configuration:   string(yamlRConfig),
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

func (c *Client) UpdateResource(resourceID string, rType string, yamlRConfig []byte) (*UpdateResourceResponse, error) {
	req := UpdateResourceRequest{
		IntegrationType: rType,
		Configuration:   string(yamlRConfig),
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

func (c *Client) DeleteResource(resourceID string) (bool, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.resourceAPI(), resourceID), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(request)
	if err != nil {
		return false, err
	}
	if response.StatusCode != 200 {
		return false, fmt.Errorf("error updating resource %s", resourceID)
	}
	return true, nil
}
