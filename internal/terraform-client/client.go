package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	defaultAdaptiveURL = "https://app.adaptive.live/api/v1"

	// requestTimeout bounds a single HTTP call. The create/delete flows poll
	// with their own retry budgets on top of this, so a hung connection can no
	// longer wedge a Terraform run indefinitely.
	requestTimeout = 60 * time.Second
)

type Client struct {
	serviceToken string
	workspaceURL string
	httpClient   *http.Client
}

// NewClient builds a Client. workspaceURL is the base workspace URL (e.g.
// https://app.adaptive.live); the /api/v1 suffix is appended automatically.
func NewClient(serviceToken, workspaceURL string) *Client {
	if workspaceURL == "" {
		workspaceURL = defaultAdaptiveURL
	} else {
		workspaceURL = fmt.Sprintf("%s/api/v1", strings.TrimRight(workspaceURL, "/"))
	}

	return &Client{
		serviceToken: serviceToken,
		workspaceURL: workspaceURL,
		httpClient:   &http.Client{Timeout: requestTimeout},
	}
}

func (c *Client) authorizationAPI() string {
	return fmt.Sprintf("%s/terraform/authorization", c.workspaceURL)
}

func (c *Client) teamAPI() string {
	return fmt.Sprintf("%s/terraform/team", c.workspaceURL)
}

func (c *Client) scriptAPI() string {
	return fmt.Sprintf("%s/terraform/script", c.workspaceURL)
}

func (c *Client) resourceAPI() string {
	return fmt.Sprintf("%s/terraform/resource", c.workspaceURL)
}

func (c *Client) sessionAPI() string {
	return fmt.Sprintf("%s/terraform/session", c.workspaceURL)
}

func (c *Client) dataProtectionAPI() string {
	return fmt.Sprintf("%s/terraform/dataprotection", c.workspaceURL)
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	tflog.Debug(ctx, "Making HTTP request", map[string]interface{}{
		"method": req.Method,
		"url":    req.URL.String(),
	})
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", c.serviceToken)
	res, err := c.httpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to make HTTP request", map[string]interface{}{
			"method": req.Method,
			"url":    req.URL.String(),
			"error":  err.Error(),
		})
		return nil, err
	}

	tflog.Debug(ctx, "HTTP response received", map[string]interface{}{
		"status_code": res.StatusCode,
		"url":         req.URL.String(),
	})
	if res.StatusCode == http.StatusUnauthorized {
		res.Body.Close()
		tflog.Error(ctx, "Authentication failed: bad token", map[string]interface{}{
			"url": req.URL.String(),
		})
		return nil, errors.New("bad token. please check your service token")
	}
	return res, nil
}

// identifiedRead is implemented by the read responses that carry the server's
// own id for the record, so readTyped can check that a 200 actually describes
// the object that was asked for.
type identifiedRead interface{ readID() string }

func (r *SessionReadResponse) readID() string        { return r.ID }
func (r *ResourceReadResponse) readID() string       { return r.ID }
func (r *AuthorizationReadResponse) readID() string  { return r.ID }
func (r *TeamReadResponse) readID() string           { return r.ID }
func (r *ScriptReadResponse) readID() string         { return r.ID }
func (r *ScheduleResponse) readID() string           { return r.ID }
func (r *DataProtectionReadResponse) readID() string { return r.ID }

// readTyped performs a GET against a /read/:id route and decodes the response
// into T. Not-found (404) is reported as (nil, nil) so callers can treat it as
// "the object no longer exists" and drop it from state rather than failing.
// It makes exactly one attempt: waiting for a status belongs in the create
// paths, not in refresh.
func readTyped[T any](c *Client, ctx context.Context, url, what, id string) (*T, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		tflog.Debug(ctx, "read returned 404, treating as deleted", map[string]interface{}{
			"type": what,
			"id":   id,
		})
		return nil, nil
	}
	// Session reads historically answer 202 Accepted; every other type uses 200.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body := readBody(resp)
		tflog.Error(ctx, "Unexpected status code on read", map[string]interface{}{
			"type":        what,
			"id":          id,
			"status_code": resp.StatusCode,
			"error_body":  body,
		})
		if body != "" {
			return nil, fmt.Errorf("error reading %s %s: %s", what, id, body)
		}
		return nil, fmt.Errorf("error reading %s %s (status %d)", what, id, resp.StatusCode)
	}

	out := new(T)
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("failed to decode %s read response. err %w", what, err)
	}
	// Fail loudly if the server hands back a different record than the one asked
	// for. Deliberately silent when the id is absent: older servers omit it, and
	// treating that as not-found would drop live resources from state on refresh.
	if v, ok := any(out).(identifiedRead); ok {
		if got := v.readID(); got != "" && got != id {
			tflog.Error(ctx, "read returned a different record than requested", map[string]interface{}{
				"type": what, "requested": id, "returned": got,
			})
			return nil, fmt.Errorf("read of %s %s returned %s instead", what, id, got)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Teams (groups)
// ---------------------------------------------------------------------------

func (c *Client) CreateTeam(ctx context.Context, name string, members, endpoints []string) (*CreateResourceResponse, error) {
	tflog.Debug(ctx, "CreateTeam called", map[string]interface{}{"name": name})

	payloadBuf, err := encode(map[string]interface{}{
		"Name":      name,
		"Members":   members,
		"Endpoints": endpoints,
	})
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/create", c.teamAPI()), payloadBuf)
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
		return nil, fmt.Errorf("duplicate group with name %s", name)
	}
	if response.StatusCode != http.StatusOK {
		if msg, derr := decodeError(ctx, response); derr == nil && msg != "" {
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("error creating group %s", name)
	}

	var resp CreateResourceResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response body. err %w", err)
	}
	tflog.Debug(ctx, "Team successfully created", map[string]interface{}{"name": name, "id": resp.ID})
	return &resp, nil
}

// ReadTeam fetches a group by ID. It returns (nil, nil) when the group no
// longer exists so callers can drop it from Terraform state.
func (c *Client) ReadTeam(ctx context.Context, id string) (*TeamReadResponse, error) {
	return readTyped[TeamReadResponse](c, ctx, fmt.Sprintf("%s/read/%s", c.teamAPI(), id), "group", id)
}

func (c *Client) UpdateTeam(ctx context.Context, id, name string, members, endpoints []string, slackChannelID string) error {
	tflog.Debug(ctx, "UpdateTeam called", map[string]interface{}{"team_id": id, "name": name})

	payloadBuf, err := encode(map[string]interface{}{
		"Name":           name,
		"Members":        members,
		"Endpoints":      endpoints,
		"SlackChannelID": slackChannelID,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/update/%s", c.teamAPI(), id), payloadBuf)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.do(ctx, request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if msg, derr := decodeError(ctx, response); derr == nil && msg != "" {
			return errors.New(msg)
		}
		return fmt.Errorf("error updating group %s", name)
	}
	return nil
}

func (c *Client) DeleteTeam(ctx context.Context, id, name string) (bool, error) {
	tflog.Debug(ctx, "DeleteTeam called", map[string]interface{}{"team_id": id, "name": name})

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/delete/%s", c.teamAPI(), id), nil)
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
		return false, fmt.Errorf("error deleting group %s", name)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// encode JSON-encodes v into a buffer suitable for an http.Request body.
func encode(v any) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, fmt.Errorf("failed to json encode request body. err %w", err)
	}
	return buf, nil
}

// readBody drains a response body and returns it as a trimmed string. Draining
// also lets the connection be reused.
func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(b))
}

func decodeError(ctx context.Context, response *http.Response) (reason string, decodeErr error) {
	var errReason ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errReason); err != nil {
		tflog.Error(ctx, "Failed to decode error response", map[string]interface{}{
			"status_code": response.StatusCode,
			"error":       err.Error(),
		})
		return "", fmt.Errorf("failed to decode response body. err %w", err)
	} else {
		reason = errReason.Error
		tflog.Debug(ctx, "Decoded error response", map[string]interface{}{
			"reason": reason,
		})
		return
	}
}
