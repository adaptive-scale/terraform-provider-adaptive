package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type VariableRequest struct {
	Name   string   `json:"name"`
	Value  string   `json:"value"`
	Labels []string `json:"labels"`
}

type VariableResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Value     string   `json:"value"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

func (c *Client) writeVariable(ctx context.Context, url string, request VariableRequest) (*VariableResponse, error) {
	payload := bytes.NewBuffer(nil)
	if err := json.NewEncoder(payload).Encode(request); err != nil {
		return nil, fmt.Errorf("encode variable request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, payload)
	if err != nil {
		return nil, err
	}
	response, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("variable request failed (status %d): %s", response.StatusCode, string(body))
	}
	var out VariableResponse
	if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode variable response: %w", err)
	}
	return &out, nil
}

func (c *Client) CreateVariable(ctx context.Context, request VariableRequest) (*VariableResponse, error) {
	return c.writeVariable(ctx, c.variableAPI()+"/create", request)
}

func (c *Client) UpdateVariable(ctx context.Context, id string, request VariableRequest) (*VariableResponse, error) {
	return c.writeVariable(ctx, fmt.Sprintf("%s/update/%s", c.variableAPI(), id), request)
}

func (c *Client) GetVariable(ctx context.Context, id string) (*VariableResponse, error) {
	var response VariableResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.variableAPI(), id), "variable", id, &response)
	if err != nil || !found {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteVariable(ctx context.Context, id string) error {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete/%s", c.variableAPI(), id), nil)
	if err != nil {
		return err
	}
	response, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("delete variable %s failed (status %d): %s", id, response.StatusCode, string(body))
	}
	return nil
}
