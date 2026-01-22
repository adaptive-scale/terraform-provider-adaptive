package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

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
	if response.StatusCode != http.StatusAccepted && response.StatusCode != http.StatusOK {

		d, _ := ioutil.ReadAll(response.Body) // drain body to allow connection reuse

		tflog.Error(ctx, "Unexpected status code reading authorization", map[string]interface{}{
			"auth_id":     authID,
			"status_code": response.StatusCode,
			"expected":    http.StatusAccepted,
			"error_body":  string(d),
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

func _readResource(ctx context.Context, c *Client, resourceID string) (map[string]interface{}, error) {
	tflog.Debug(ctx, "Reading resource", map[string]interface{}{
		"resource_id": resourceID,
	})
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.resourceAPI(), resourceID), nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create HTTP request for reading resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		tflog.Error(ctx, "Failed to request adaptive api for resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	if response.StatusCode != http.StatusAccepted {

		d, _ := ioutil.ReadAll(response.Body) // drain body to allow connection reuse

		tflog.Error(ctx, "Unexpected status code reading resource", map[string]interface{}{
			"resource_id": resourceID,
			"status_code": response.StatusCode,
			"expected":    http.StatusAccepted,
			"error_body":  string(d),
		})
		return nil, fmt.Errorf("error read resource %s", resourceID)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		tflog.Error(ctx, "Failed to decode response body for resource", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Successfully read resource", map[string]interface{}{
		"resource_id": resourceID,
		"response":    fmt.Sprintf("%+v", resp),
	})
	return resp, nil
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
