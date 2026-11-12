package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrNotFound        = errors.New("adaptive object not found")
	ErrReadUnsupported = errors.New("server does not support this read endpoint")
)

// ResponseValue prefers canonical keys while accepting the spellings emitted
// by older releases. Presence matters: false, zero and empty lists are values.
func ResponseValue(data map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func (c *Client) readObject(ctx context.Context, api, id string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/read/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}
		// Gin's missing-route response must not evict an existing object from
		// state on a server predating the team/script/resource read handlers.
		if resp.StatusCode == http.StatusNotFound && strings.TrimSpace(string(body)) == "404 page not found" {
			return nil, ErrReadUnsupported
		}
		if resp.StatusCode == http.StatusNotFound && json.Valid(body) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading adaptive object %s: HTTP %d", id, resp.StatusCode)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding adaptive read response: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("empty adaptive read response")
	}
	return data, nil
}

// Refresh never waits for readiness: paused/failed endpoints still exist.
// Readiness polling is reserved for create callers and tolerates the resource
// response introduced in v0.9, which has no status field at all.
func (c *Client) readWithStatus(ctx context.Context, api, id string, wait bool) (map[string]interface{}, error) {
	for attempt := 0; ; attempt++ {
		data, err := c.readObject(ctx, api, id)
		if err != nil {
			return nil, err
		}
		if !wait {
			return data, nil
		}
		value, exists := ResponseValue(data, "status", "Status")
		if !exists {
			return data, nil
		}
		status, ok := value.(string)
		if !ok {
			return nil, errors.New("invalid adaptive status response")
		}
		switch strings.ToLower(status) {
		case "creating":
			if attempt >= 29 {
				return nil, fmt.Errorf("timed out waiting for adaptive object %s", id)
			}
		case "failed", "failed-to-restart":
			return nil, fmt.Errorf("adaptive object %s has status %s", id, status)
		default:
			return data, nil
		}
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) ReadTeam(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.readObject(ctx, c.teamAPI(), id)
}

func (c *Client) ReadScript(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.readObject(ctx, c.scriptAPI(), id)
}

func (c *Client) ReadSchedule(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.readObject(ctx, c.scheduleAPI(), id)
}
