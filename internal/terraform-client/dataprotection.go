package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// writeDataProtection posts a masking policy to create or update and decodes
// the full policy the server echoes back.
func (c *Client) writeDataProtection(ctx context.Context, url string, req DataProtectionRequest) (*DataProtectionReadResponse, error) {
	payloadBuf, err := encode(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", url, payloadBuf)
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
		tflog.Error(ctx, "data protection write failed", map[string]interface{}{
			"url":         url,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return nil, fmt.Errorf("error writing data protection for %s: %s", req.ResourceName, body)
		}
		return nil, fmt.Errorf("error writing data protection for %s (status %d)", req.ResourceName, response.StatusCode)
	}

	var resp DataProtectionReadResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

// CreateDataProtection creates (or adopts — the server upserts per resource) a
// masking policy. The returned ID is the resource's integration ID.
func (c *Client) CreateDataProtection(ctx context.Context, req DataProtectionRequest) (*DataProtectionReadResponse, error) {
	tflog.Debug(ctx, "CreateDataProtection called", map[string]interface{}{"resource": req.ResourceName})
	return c.writeDataProtection(ctx, fmt.Sprintf("%s/create", c.dataProtectionAPI()), req)
}

func (c *Client) UpdateDataProtection(ctx context.Context, id string, req DataProtectionRequest) (*DataProtectionReadResponse, error) {
	tflog.Debug(ctx, "UpdateDataProtection called", map[string]interface{}{"id": id, "resource": req.ResourceName})
	return c.writeDataProtection(ctx, fmt.Sprintf("%s/update/%s", c.dataProtectionAPI(), id), req)
}

// ReadDataProtection fetches a masking policy by resource (integration) ID. It
// returns (nil, nil) when the resource or its policy no longer exists.
func (c *Client) ReadDataProtection(ctx context.Context, id string) (*DataProtectionReadResponse, error) {
	return readTyped[DataProtectionReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.dataProtectionAPI(), id), "data protection", id)
}

// DeleteDataProtection turns masking off for the resource — the server applies
// the documented "off" shape (scoped: false, masks: []) rather than deleting
// rows, matching the admin CLI's behaviour. Idempotent.
func (c *Client) DeleteDataProtection(ctx context.Context, id string) error {
	tflog.Debug(ctx, "DeleteDataProtection called", map[string]interface{}{"id": id})

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.dataProtectionAPI(), id), nil)
	if err != nil {
		return err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body := readBody(response)
		tflog.Error(ctx, "Failed to delete data protection", map[string]interface{}{
			"id":          id,
			"status_code": response.StatusCode,
			"reason":      body,
		})
		if body != "" {
			return fmt.Errorf("error deleting data protection %s: %s", id, body)
		}
		return fmt.Errorf("error deleting data protection %s (status %d)", id, response.StatusCode)
	}
	return nil
}
