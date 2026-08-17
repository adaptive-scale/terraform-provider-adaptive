package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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

// ReadAuthorization fetches an authorization by ID. It returns (nil, nil) when
// the authorization no longer exists so callers can drop it from state.
func (c *Client) ReadAuthorization(ctx context.Context, authID string) (*AuthorizationReadResponse, error) {
	return readTyped[AuthorizationReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.authorizationAPI(), authID), "authorization", authID)
}

// waitForAuthorization polls until the authorization leaves the "creating"
// status. A not-found read is retried: right after create the row may not be
// readable yet.
func (c *Client) waitForAuthorization(ctx context.Context, authID string) (*AuthorizationReadResponse, error) {
	return Do(
		func() (*AuthorizationReadResponse, error) {
			return c.ReadAuthorization(ctx, authID)
		}, RetryLimit(20), Sleep(10*time.Second), RetryResultChecker(func(intermedResult any) bool {
			res, ok := intermedResult.(*AuthorizationReadResponse)
			if !ok || res == nil {
				return true
			}
			// Stop once the status is terminal ("created", "failed", ...).
			return strings.ToLower(res.Status) == "creating"
		}))
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
	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.authorizationAPI()), payloadBuf)
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
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != http.StatusOK {
		errReason := readBody(_response)
		tflog.Error(ctx, "Failed to create authorization", map[string]interface{}{
			"name":        req.AuthorizationName,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error creating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response CreateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}

	// Wait for it to become functional before reporting success.
	if _, err := c.waitForAuthorization(ctx, response.ID); err != nil {
		tflog.Error(ctx, "Failed to verify authorization after creation", map[string]interface{}{
			"id":    response.ID,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to create authorization %s: %w", response.ID, err)
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
	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.authorizationAPI(), authID), payloadBuf)
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
		return nil, fmt.Errorf("duplicate authorization with name %s", req.AuthorizationName)
	}
	if _response.StatusCode != http.StatusOK {
		errReason := readBody(_response)
		tflog.Error(ctx, "Failed to update authorization", map[string]interface{}{
			"auth_id":     authID,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		return nil, fmt.Errorf("error updating authorization %s, reason %s", req.AuthorizationName, errReason)
	}

	var response UpdateAuthorizationResponse
	if err := json.NewDecoder(_response.Body).Decode(&response); err != nil {
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
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to delete authorization", map[string]interface{}{
			"auth_id":       authID,
			"status_code":   response.StatusCode,
			"response_body": body,
		})
		return false, fmt.Errorf("error deleting authorization %s: %s", authID, body)
	}
	tflog.Debug(ctx, "Authorization successfully deleted", map[string]interface{}{
		"auth_id": authID,
	})
	return true, nil
}
