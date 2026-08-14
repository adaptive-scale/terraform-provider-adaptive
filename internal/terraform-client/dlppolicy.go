package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DLPPolicyRequest is the JSON body consumed by the backend Terraform DLP
// policy API (POST /api/v1/terraform/dlppolicy). The endpoint upserts by
// name. Field names map 1:1 to the server contract.
type DLPPolicyRequest struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Enabled               bool     `json:"enabled"`
	BlockScreenshot       bool     `json:"blockScreenshot"`
	BlockClipboardCopy    bool     `json:"blockClipboardCopy"`
	BlockClipboardPaste   bool     `json:"blockClipboardPaste"`
	BlockFileTransfer     bool     `json:"blockFileTransfer"`
	BlockBulkExport       bool     `json:"blockBulkExport"`
	MaxExportRows         int      `json:"maxExportRows"`
	BlockSessionLogExport bool     `json:"blockSessionLogExport"`
	BlockedSurfaces       []string `json:"blockedSurfaces"`
	Exceptions            []string `json:"exceptions"`
	AppliesToLabels       []string `json:"appliesToLabels"`
	WorkspaceDefault      bool     `json:"workspace_default"`
}

// DLPPolicyResponse is the policy JSON the backend returns, i.e. the request
// shape plus the server-side id.
type DLPPolicyResponse struct {
	ID string `json:"id"`
	DLPPolicyRequest
}

func (c *Client) dlpPolicyAPI() string {
	return fmt.Sprintf("%s/terraform/dlppolicy", c.workspaceURL)
}

// dlpPolicyErr extracts the most specific error reason the backend reported.
// Exception expressions (expr-lang) are validated server-side, so validation
// messages must reach the practitioner verbatim.
func dlpPolicyErr(ctx context.Context, response *http.Response, action, name string) error {
	body, readErr := io.ReadAll(response.Body)
	if readErr == nil && len(body) > 0 {
		var errReason ErrorResponse
		if err := json.Unmarshal(body, &errReason); err == nil && errReason.Error != "" {
			return errors.New(errReason.Error)
		}
		if msg := strings.TrimSpace(string(body)); msg != "" {
			return errors.New(msg)
		}
	}
	tflog.Error(ctx, "dlp policy request failed", map[string]interface{}{
		"action":      action,
		"name":        name,
		"status_code": response.StatusCode,
	})
	return fmt.Errorf("error %s dlp policy %q (status %d)", action, name, response.StatusCode)
}

// UpsertDLPPolicy creates or updates a DLP policy. The backend upserts by
// policy name.
func (c *Client) UpsertDLPPolicy(ctx context.Context, req *DLPPolicyRequest) (*DLPPolicyResponse, error) {
	tflog.Debug(ctx, "UpsertDLPPolicy called", map[string]interface{}{"name": req.Name})

	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to json encode request body. err %w", err)
	}

	request, err := http.NewRequest("POST", c.dlpPolicyAPI(), payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, dlpPolicyErr(ctx, response, "writing", req.Name)
	}

	var resp DLPPolicyResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

// GetDLPPolicy reads a DLP policy by name. It returns (nil, nil) when the
// policy no longer exists so callers can drop it from Terraform state.
func (c *Client) GetDLPPolicy(ctx context.Context, name string) (*DLPPolicyResponse, error) {
	tflog.Debug(ctx, "GetDLPPolicy called", map[string]interface{}{"name": name})

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.dlpPolicyAPI(), url.PathEscape(name)), nil)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, dlpPolicyErr(ctx, response, "reading", name)
	}

	var resp DLPPolicyResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

// DeleteDLPPolicy removes a DLP policy by name. The backend also clears
// endpoint/workspace references to the policy. A missing policy still
// reports success so deletes stay idempotent.
func (c *Client) DeleteDLPPolicy(ctx context.Context, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteDLPPolicy called", map[string]interface{}{"name": name})

	request, err := http.NewRequest("DELETE", fmt.Sprintf("%s/%s", c.dlpPolicyAPI(), url.PathEscape(name)), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return true, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, dlpPolicyErr(ctx, response, "deleting", name)
	}
	return true, nil
}
