package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type DataProtectionRequest struct {
	ResourceName string               `json:"resourceName"`
	Scoped       bool                 `json:"scoped"`
	Masks        []DataProtectionMask `json:"masks"`
}

type DataProtectionMask struct {
	DatabaseName string                `json:"databaseName"`
	Tables       []DataProtectionTable `json:"tables"`
}

type DataProtectionTable struct {
	TableName     string                 `json:"tableName"`
	Schema        string                 `json:"schema,omitempty"`
	Masked        bool                   `json:"masked"`
	MaskedColumns []DataProtectionColumn `json:"maskedColumns"`
}

type DataProtectionColumn struct {
	ColumnName  string `json:"columnName"`
	MaskingType string `json:"maskingType"`
}

type DataProtectionResponse struct {
	DataProtectionRequest
	ID                string `json:"id"`
	AuthorizationName string `json:"authorizationName"`
	Status            string `json:"status"`
	LastAppliedAt     string `json:"lastAppliedAt"`
}

func (c *Client) dataProtectionAPI() string { return c.workspaceURL + "/terraform/dataprotection" }

func (c *Client) ReadDataProtection(ctx context.Context, id string) (*DataProtectionResponse, error) {
	data, err := c.readObject(ctx, c.dataProtectionAPI(), id)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var result DataProtectionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) WriteDataProtection(ctx context.Context, id string, policy *DataProtectionRequest) (*DataProtectionResponse, error) {
	endpoint := c.dataProtectionAPI() + "/create"
	if id != "" {
		endpoint = c.dataProtectionAPI() + "/update/" + url.PathEscape(id)
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(policy); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		reason, _ := decodeError(ctx, resp)
		return nil, fmt.Errorf("writing data protection for %q: HTTP %d: %s", policy.ResourceName, resp.StatusCode, reason)
	}
	var result DataProtectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.ID == "" {
		return nil, fmt.Errorf("data protection response has no resource ID")
	}
	return &result, nil
}

func (c *Client) DeleteDataProtection(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.dataProtectionAPI()+"/delete/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		reason, _ := decodeError(ctx, resp)
		return fmt.Errorf("disabling data protection: HTTP %d: %s", resp.StatusCode, reason)
	}
	return nil
}
