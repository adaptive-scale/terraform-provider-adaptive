package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ReadResource fetches an integration (resource) by ID. Secret configuration
// values are stripped server-side; their keys are listed in RedactedKeys. It
// returns (nil, nil) when the resource no longer exists so callers can drop it
// from Terraform state.
func (c *Client) ReadResource(ctx context.Context, resourceID string) (*ResourceReadResponse, error) {
	return readTyped[ResourceReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.resourceAPI(), resourceID), "resource", resourceID)
}

// Resources / Integrations
func (c *Client) CreateResource(
	ctx context.Context,
	name, rType string,
	yamlRConfig []byte,
	tags []string,
	defaultCluster string,
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
		DefaultCluster:  defaultCluster,
	}

	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.resourceAPI()), payloadBuf)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("duplicate resource with name %s", name)
	}
	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to create resource", map[string]interface{}{
			"name":        req.Name,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return nil, fmt.Errorf("error creating resource %s: %s", req.Name, body)
		}
		return nil, fmt.Errorf("error creating resource %s", req.Name)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
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
	defaultCluster string,
) (*UpdateResourceResponse, error) {
	tflog.Debug(ctx, "UpdateResource called", map[string]interface{}{
		"resource_id": resourceID,
		"type":        rType,
	})
	req := UpdateResourceRequest{
		IntegrationType: rType,
		Configuration:   string(yamlRConfig),
		UserTags:        tags,
		DefaultCluster:  defaultCluster,
	}

	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.resourceAPI(), resourceID), payloadBuf)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to update resource", map[string]interface{}{
			"resource_id": resourceID,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return nil, fmt.Errorf("error updating resource %s: %s", resourceID, body)
		}
		return nil, fmt.Errorf("error updating resource %s", resourceID)
	}

	var updateResourceResponse UpdateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&updateResourceResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
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
		return false, err
	}

	_response, err := c.do(ctx, request)
	if err != nil {
		return false, err
	}
	defer _response.Body.Close()

	if _response.StatusCode != http.StatusOK {
		errReason := readBody(_response)
		tflog.Error(ctx, "Failed to delete resource", map[string]interface{}{
			"name":        resourceName,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		msg := fmt.Sprintf("error deleting resource %s", resourceName)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, fmt.Errorf("%s", msg)
	}
	tflog.Debug(ctx, "Resource successfully deleted", map[string]interface{}{
		"name": resourceName,
	})
	return true, nil
}
