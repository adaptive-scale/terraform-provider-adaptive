package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// State-refresh reads.
//
// These are deliberately separate from ReadResource / ReadSession /
// ReadAuthorization, which look like reads but are provisioning waiters: they
// poll on a 10s interval up to 20-30 times and return an error unless the object
// settles on Status == "created". That behaviour is right after a create and
// wrong for a refresh, where a single request must answer "what does the backend
// hold right now", and where an object that has been deleted out of band has to
// come back as not-found rather than as an error.
//
// Every function here returns (nil, nil) when the object no longer exists, so
// callers can d.SetId("") and let Terraform plan a recreate.

// readInto performs a single GET and decodes the body into out.
//
// It reports found=false on 404. 202 is accepted alongside 200 because the
// session read has always answered 202 and released provider versions depend on
// that; treating it as an error here would break refresh against every backend.
func (c *Client) readInto(ctx context.Context, url, kind, id string, out interface{}) (bool, error) {
	tflog.Debug(ctx, "reading "+kind, map[string]interface{}{"id": id, "url": url})

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	response, err := c.do(ctx, request)
	if err != nil {
		return false, fmt.Errorf("failed to request adaptive api. err %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		tflog.Debug(ctx, kind+" not found, treating as deleted", map[string]interface{}{"id": id})
		return false, nil
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(response.Body) // drain to allow connection reuse
		tflog.Error(ctx, "unexpected status reading "+kind, map[string]interface{}{
			"id":          id,
			"status_code": response.StatusCode,
			"error_body":  string(body),
		})
		return false, fmt.Errorf("error reading %s %s (status %d)", kind, id, response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return false, fmt.Errorf("failed to decode %s response body. err %w", kind, err)
	}
	return true, nil
}

// ResourceResponse mirrors handler.TerraformResourceResponse.
//
// Configuration carries the stored settings blob with credentials removed, and
// RedactedKeys names what was removed. A provider must leave the attributes
// named in RedactedKeys at their prior state value: they are write-only, and
// diffing them against an absent value would propose clearing a live credential.
type ResourceResponse struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	IntegrationType string                 `json:"integrationType"`
	UserTags        []string               `json:"userTags"`
	DefaultCluster  string                 `json:"defaultCluster,omitempty"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
	RedactedKeys    []string               `json:"redactedKeys,omitempty"`
}

// GetResource reads a resource for refresh. Returns (nil, nil) if it is gone.
func (c *Client) GetResource(ctx context.Context, id string) (*ResourceResponse, error) {
	var resp ResourceResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.resourceAPI(), id), "resource", id, &resp)
	if err != nil || !found {
		return nil, err
	}
	return &resp, nil
}

// SessionResponse mirrors handler.TerraformSessionResponse. Resource, Cluster,
// Authorization and Groups come back as names, matching what the write path
// accepts, so a refresh compares like with like.
type SessionResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Resource        string   `json:"resource"`
	Cluster         string   `json:"cluster,omitempty"`
	Authorization   string   `json:"authorization,omitempty"`
	Status          string   `json:"status"`
	SessionType     string   `json:"sessionType"`
	TTL             string   `json:"ttl,omitempty"`
	IdleTimeout     string   `json:"idleTimeout,omitempty"`
	PauseTimeout    string   `json:"pauseTimeout,omitempty"`
	Memory          string   `json:"memory,omitempty"`
	CPU             string   `json:"cpu,omitempty"`
	IsJITEnabled    bool     `json:"isJitEnabled"`
	SessionUsers    []string `json:"sessionUsers"`
	AccessApprovers []string `json:"accessApprovers"`
	Groups          []string `json:"groups"`
	UserTags        []string `json:"userTags"`

	ScriptOnlyAccess bool `json:"scriptOnlyAccess"`
}

// GetSession reads an endpoint for refresh. Returns (nil, nil) if it is gone.
func (c *Client) GetSession(ctx context.Context, id string) (*SessionResponse, error) {
	var resp SessionResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.sessionAPI(), id), "endpoint", id, &resp)
	if err != nil || !found {
		return nil, err
	}
	return &resp, nil
}

// AuthorizationResponse mirrors handler.TerraformAuthorizationResponse.
type AuthorizationResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ResourceType string `json:"resourceType"`
	Permissions  string `json:"permissions"`
	Status       string `json:"status"`
}

// GetAuthorization reads an authorization for refresh, without the create-time
// polling ReadAuthorization does. Returns (nil, nil) if it is gone.
func (c *Client) GetAuthorization(ctx context.Context, id string) (*AuthorizationResponse, error) {
	var resp AuthorizationResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.authorizationAPI(), id), "authorization", id, &resp)
	if err != nil || !found {
		return nil, err
	}
	return &resp, nil
}

// TeamResponse mirrors handler.TerraformTeamResponse. Members are emails and
// endpoints are endpoint names, matching the write path.
type TeamResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Members        []string `json:"members"`
	Endpoints      []string `json:"endpoints"`
	SlackChannelID string   `json:"slackChannelId,omitempty"`
}

// GetTeamDetail reads a group for refresh. Returns (nil, nil) if it is gone.
//
// Named apart from the pre-existing GetTeam, which hits /terraform/team/{id} -
// a route with no /read/ segment that only ever returned {id} and so could not
// populate name, members or endpoints.
func (c *Client) GetTeamDetail(ctx context.Context, id string) (*TeamResponse, error) {
	var resp TeamResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.teamAPI(), id), "group", id, &resp)
	if err != nil || !found {
		return nil, err
	}
	return &resp, nil
}

// ScriptResponse mirrors handler.TerraformScriptResponse.
//
// There is no Command field. A script body routinely embeds credentials, so the
// backend withholds it and sets CommandOmitted; the provider must leave the
// command attribute at its prior state value.
type ScriptResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Endpoint       string `json:"endpoint"`
	Description    string `json:"description,omitempty"`
	CommandOmitted bool   `json:"commandOmitted"`
}

// GetScript reads a script for refresh. Returns (nil, nil) if it is gone.
func (c *Client) GetScript(ctx context.Context, id string) (*ScriptResponse, error) {
	var resp ScriptResponse
	found, err := c.readInto(ctx, fmt.Sprintf("%s/read/%s", c.scriptAPI(), id), "script", id, &resp)
	if err != nil || !found {
		return nil, err
	}
	return &resp, nil
}
