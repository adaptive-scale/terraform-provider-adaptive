package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *Client) ReadResource(ctx context.Context, resourceID string, waitForStatus bool) (any, error) {
	tflog.Debug(ctx, "ReadResource called", map[string]interface{}{
		"resource_id":     resourceID,
		"wait_for_status": waitForStatus,
	})
	timeout := time.Second * 10
	retryForStatus := 20
	if waitForStatus {
		retryForStatus = 20
	}

	resp, err := Do(
		func() (map[string]interface{}, error) {
			return _readResource(ctx, c, resourceID)
		}, RetryLimit(retryForStatus), Sleep(timeout), RetryResultChecker(func(intermedResult any) bool {
			if res, ok := intermedResult.(map[string]interface{}); !ok {
				tflog.Warn(ctx, "Resource result has bad data format", map[string]interface{}{
					"resource_id": resourceID,
				})
				return true
			} else {
				if res != nil {
					if status, okk := res["Status"].(string); !okk {
						tflog.Warn(ctx, "Resource status field missing or not a string", map[string]interface{}{
							"resource_id": resourceID,
						})
						return true
					} else {
						statusLower := strings.ToLower(status)
						tflog.Debug(ctx, "Resource status check", map[string]interface{}{
							"resource_id": resourceID,
							"status":      status,
							"is_creating": statusLower == "creating",
						})
						// Will return false, when state is among final states like "created" or "failed"
						return statusLower == "creating"
					}
				}
				tflog.Warn(ctx, "Resource result is nil", map[string]interface{}{
					"resource_id": resourceID,
				})
				return true
			}
		}))
	if err != nil {
		tflog.Error(ctx, "Failed to read resource after retries", map[string]interface{}{
			"resource_id": resourceID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("could not read resource %s %w", resourceID, err)
	}
	finalStatus := strings.ToLower(resp["Status"].(string))
	if finalStatus != "created" {
		tflog.Error(ctx, "Resource not in created status", map[string]interface{}{
			"resource_id": resourceID,
			"status":      resp["Status"],
		})
		return nil, fmt.Errorf("error read resource %s", resourceID)
	}
	tflog.Debug(ctx, "Resource successfully created", map[string]interface{}{
		"resource_id": resourceID,
	})
	return resp, nil

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
