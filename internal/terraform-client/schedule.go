package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ScheduleRequest is the flat JSON shape consumed by the backend Terraform
// schedule API (handler.TerraformCreateScheduleRequest). Field names map 1:1.
type ScheduleRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ScheduleType string `json:"scheduleType"`
	IsActive     *bool  `json:"isActive,omitempty"`

	AllDay      bool `json:"allDay,omitempty"`
	StartHour   int  `json:"startHour"`
	StartMinute int  `json:"startMinute"`
	EndHour     int  `json:"endHour"`
	EndMinute   int  `json:"endMinute"`

	Weekdays      []string `json:"weekdays,omitempty"`
	StartDay      int      `json:"startDay,omitempty"`
	EndDay        int      `json:"endDay,omitempty"`
	SpecificDates []string `json:"specificDates,omitempty"`

	Users     []string `json:"users,omitempty"`
	Teams     []string `json:"teams,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`

	ExpiresAt     *string `json:"expiresAt,omitempty"`
	MaxAccessTime *int    `json:"maxAccessTime,omitempty"`
	Timezone      string  `json:"timezone,omitempty"`
}

// ScheduleResponse mirrors handler.TerraformScheduleResponse. On a 400 the
// backend reuses this shape to report names it could not resolve to IDs.
type ScheduleResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ScheduleType    string   `json:"scheduleType"`
	IsActive        bool     `json:"isActive"`
	AllDay          bool     `json:"allDay"`
	MappedEndpoints []string `json:"mappedEndpoints,omitempty"`
	Timezone        string   `json:"timezone,omitempty"`

	UnresolvedUsers     []string `json:"unresolvedUsers,omitempty"`
	UnresolvedTeams     []string `json:"unresolvedTeams,omitempty"`
	UnresolvedEndpoints []string `json:"unresolvedEndpoints,omitempty"`
}

func (c *Client) scheduleAPI() string {
	return fmt.Sprintf("%s/terraform/schedule", c.workspaceURL)
}

// unresolvedErr turns a backend 400 with unresolved names into a useful message.
func (r *ScheduleResponse) unresolvedErr() error {
	var parts []string
	if len(r.UnresolvedUsers) > 0 {
		parts = append(parts, fmt.Sprintf("users %v", r.UnresolvedUsers))
	}
	if len(r.UnresolvedTeams) > 0 {
		parts = append(parts, fmt.Sprintf("teams %v", r.UnresolvedTeams))
	}
	if len(r.UnresolvedEndpoints) > 0 {
		parts = append(parts, fmt.Sprintf("endpoints %v", r.UnresolvedEndpoints))
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("could not resolve %s — check the names exist in this workspace", strings.Join(parts, ", "))
}

func (c *Client) writeSchedule(ctx context.Context, method, url string, req *ScheduleRequest) (*ScheduleResponse, error) {
	payloadBuf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(payloadBuf).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to json encode request body. err %w", err)
	}

	request, err := http.NewRequest(method, url, payloadBuf)
	if err != nil {
		return nil, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var resp ScheduleResponse
		if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
			return nil, fmt.Errorf("failed to decode response body. err %w", err)
		}
		return &resp, nil
	}

	// Non-200: try to surface the most specific reason we can.
	if response.StatusCode == http.StatusBadRequest {
		var resp ScheduleResponse
		if err := json.NewDecoder(response.Body).Decode(&resp); err == nil {
			if unresolved := resp.unresolvedErr(); unresolved != nil {
				return nil, unresolved
			}
		}
	}
	if msg, derr := decodeError(ctx, response); derr == nil && msg != "" {
		return nil, fmt.Errorf("schedule %q: %s", req.Name, msg)
	}
	tflog.Error(ctx, "schedule write failed", map[string]interface{}{
		"url":         url,
		"status_code": response.StatusCode,
	})
	return nil, fmt.Errorf("error writing schedule %q (status %d)", req.Name, response.StatusCode)
}

func (c *Client) CreateSchedule(ctx context.Context, req *ScheduleRequest) (*ScheduleResponse, error) {
	tflog.Debug(ctx, "CreateSchedule called", map[string]interface{}{"name": req.Name})
	return c.writeSchedule(ctx, "POST", fmt.Sprintf("%s/create", c.scheduleAPI()), req)
}

func (c *Client) UpdateSchedule(ctx context.Context, id string, req *ScheduleRequest) (*ScheduleResponse, error) {
	tflog.Debug(ctx, "UpdateSchedule called", map[string]interface{}{"id": id, "name": req.Name})
	return c.writeSchedule(ctx, "POST", fmt.Sprintf("%s/update/%s", c.scheduleAPI(), id), req)
}

// GetSchedule reads a schedule. It returns (nil, nil) when the schedule no
// longer exists so callers can drop it from Terraform state.
func (c *Client) GetSchedule(ctx context.Context, id string) (*ScheduleResponse, error) {
	tflog.Debug(ctx, "GetSchedule called", map[string]interface{}{"id": id})
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/read/%s", c.scheduleAPI(), id), nil)
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
		return nil, fmt.Errorf("error reading schedule %s (status %d)", id, response.StatusCode)
	}

	var resp ScheduleResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	return &resp, nil
}

// DeleteSchedule removes a schedule. The backend delete is idempotent, so a
// missing schedule still reports success.
func (c *Client) DeleteSchedule(ctx context.Context, id, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteSchedule called", map[string]interface{}{"id": id, "name": name})
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.scheduleAPI(), id), nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if msg, derr := decodeError(ctx, response); derr == nil && msg != "" {
			return false, errors.New(msg)
		}
		return false, fmt.Errorf("error deleting schedule %q (status %d)", name, response.StatusCode)
	}
	return true, nil
}
