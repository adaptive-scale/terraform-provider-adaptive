package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *Client) CreateScript(ctx context.Context, req ScriptRequest) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "CreateScript called", map[string]interface{}{
		"name":     req.Name,
		"endpoint": req.Endpoint,
	})

	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.scriptAPI()), payloadBuf)
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
		return nil, fmt.Errorf("duplicate script with name %s", req.Name)
	}
	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to create script", map[string]interface{}{
			"name":        req.Name,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return nil, fmt.Errorf("error creating script %s: %s", req.Name, body)
		}
		return nil, fmt.Errorf("error creating script %s", req.Name)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Script successfully created", map[string]interface{}{
		"name": req.Name,
		"id":   resp.ID,
	})
	return &resp, nil
}

// ReadScript fetches a script by ID. The script body (command) is write-only
// and never returned; CommandOmitted is set instead and CommandDigest carries
// an opaque fingerprint of it. Returns (nil, nil) when the script is gone.
func (c *Client) ReadScript(ctx context.Context, id string) (*ScriptReadResponse, error) {
	return readTyped[ScriptReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.scriptAPI(), id), "script", id)
}

func (c *Client) UpdateScript(ctx context.Context, id string, req ScriptRequest) error {
	tflog.Debug(ctx, "UpdateScript called", map[string]interface{}{
		"script_id": id,
		"name":      req.Name,
	})

	payloadBuf, err := encode(req)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.scriptAPI(), id), payloadBuf)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusConflict {
		return fmt.Errorf("duplicate script with name %s", req.Name)
	}
	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to update script", map[string]interface{}{
			"script_id":   id,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return fmt.Errorf("error updating script %s: %s", req.Name, body)
		}
		return fmt.Errorf("error updating script %s", req.Name)
	}
	tflog.Debug(ctx, "Script successfully updated", map[string]interface{}{
		"script_id": id,
	})
	return nil
}

func (c *Client) DeleteScript(ctx context.Context, id, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteScript called", map[string]interface{}{
		"script_id": id,
		"name":      name,
	})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.scriptAPI(), id), nil)
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
		tflog.Error(ctx, "Failed to delete script", map[string]interface{}{
			"name":        name,
			"status_code": _response.StatusCode,
			"reason":      errReason,
		})
		msg := fmt.Sprintf("error deleting script %s", name)
		if len(errReason) > 0 {
			msg += fmt.Sprintf(". reason %s", errReason)
		}
		return false, fmt.Errorf("%s", msg)
	}
	tflog.Debug(ctx, "Script successfully deleted", map[string]interface{}{
		"name": name,
	})
	return true, nil
}
