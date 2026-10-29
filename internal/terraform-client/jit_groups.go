package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Client) RequireJITApproverGroups(ctx context.Context, requireEnabled bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.workspaceURL+"/cli/admin/jit.approver-groups", nil)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return fmt.Errorf("cannot verify JIT approver group support; no changes submitted: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JIT approver groups are unsupported (HTTP %d); no changes submitted", resp.StatusCode)
	}
	var capability struct {
		Supported bool `json:"supported"`
		Enabled   bool `json:"enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&capability); err != nil {
		return err
	}
	if !capability.Supported || (requireEnabled && !capability.Enabled) {
		return fmt.Errorf("JIT approver group writes are unavailable; no changes submitted")
	}
	return nil
}

func (c *Client) ReadJITApproverGroups(ctx context.Context, id string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sessionAPI()+"/read/"+id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read endpoint JIT groups: HTTP %d", resp.StatusCode)
	}
	var value struct {
		Groups *[]string `json:"accessApproverGroups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&value); err != nil {
		return nil, err
	}
	if value.Groups == nil {
		return nil, fmt.Errorf("server omitted JIT approver groups; upgrade the server before managing this field")
	}
	return *value.Groups, nil
}
