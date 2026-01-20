package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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
type CreateAuthorizationRequest struct {
	AuthorizationName string `json:"name"`
	Resource          string `json:"resource"`
	Description       string `json:"description"`
	Permissions       string `json:"permissions"`
}

type CreateAuthorizationResponse struct {
	ID string `json:"id"`
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
