package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// fakeAPI serves a canned JSON body for every request and records the paths it
// was called on, so a component's Read/Create can be driven end to end through
// the real client.
type fakeAPI struct {
	mu    sync.Mutex
	paths []string
	body  []map[string]any
}

func (f *fakeAPI) start(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *adaptive.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.body = append(f.body, body)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return adaptive.NewClient("test-token", srv.URL)
}

func (f *fakeAPI) sawPath(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.paths {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// Authorization
// ---------------------------------------------------------------------------

// The read DTO is camelCase; the previous implementation looked for a
// "resource_type" key that the API never sends, so resource_type and
// permissions silently never refreshed.
func TestAuthorizationReadMapsServerFields(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"a-1","name":"read-only","description":"analysts",
			"resourceType":"postgres","permissions":"SELECT","status":"created"}`))
	})

	res := ResourceAdaptiveAuthorization()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":          "stale",
		"resource_type": "mysql",
		"permissions":   "ALL",
	})
	d.SetId("a-1")

	if diags := ResourceAdaptiveAuthorizationRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("name").(string); got != "read-only" {
		t.Errorf("name: got %q", got)
	}
	if got := d.Get("resource_type").(string); got != "postgres" {
		t.Errorf("resource_type: got %q", got)
	}
	if got := d.Get("permissions").(string); got != "SELECT" {
		t.Errorf("permissions: got %q", got)
	}
	if got := d.Get("description").(string); got != "analysts" {
		t.Errorf("description: got %q", got)
	}
}

func TestAuthorizationReadDropsDeleted(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, notFound)

	res := ResourceAdaptiveAuthorization()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "gone"})
	d.SetId("a-1")

	if diags := ResourceAdaptiveAuthorizationRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("a deleted authorization must not fail the refresh: %+v", diags)
	}
	if d.Id() != "" {
		t.Errorf("id should be cleared so Terraform recreates it, got %q", d.Id())
	}
}

// ---------------------------------------------------------------------------
// Team
// ---------------------------------------------------------------------------

// The group create route has no slack field, so a configured channel only
// lands if create is followed by an update.
func TestTeamCreateAppliesSlackChannelWithFollowUpUpdate(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t-1"}`))
	})

	res := ResourceAdaptiveTeam()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":             "oncall",
		"members":          []interface{}{"a@x.com"},
		"slack_channel_id": "C123",
	})

	if diags := ResourceAdaptiveTeamCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create: %+v", diags)
	}
	if d.Id() != "t-1" {
		t.Fatalf("id: got %q", d.Id())
	}
	if !api.sawPath("/terraform/team/create") || !api.sawPath("/terraform/team/update/t-1") {
		t.Fatalf("expected create then update, saw %v", api.paths)
	}
	if got := api.body[1]["SlackChannelID"]; got != "C123" {
		t.Errorf("slack channel not sent on the follow-up update: %v", api.body[1])
	}
}

// Without a slack channel there is nothing to apply, so the extra round trip
// must not happen.
func TestTeamCreateSkipsUpdateWithoutSlackChannel(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t-1"}`))
	})

	res := ResourceAdaptiveTeam()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":    "oncall",
		"members": []interface{}{"a@x.com"},
	})

	if diags := ResourceAdaptiveTeamCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create: %+v", diags)
	}
	if api.sawPath("/terraform/team/update/") {
		t.Errorf("unexpected update call: %v", api.paths)
	}
}

func TestTeamReadKeepsConfiguredOrderingAndIgnoresUnsetSlack(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		// Same members as the configuration, different order; plus a slack
		// channel the configuration never asked for.
		_, _ = w.Write([]byte(`{"id":"t-1","name":"oncall","members":["b@x.com","a@x.com"],
			"endpoints":["ep2"],"slackChannelId":"C999"}`))
	})

	res := ResourceAdaptiveTeam()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":      "oncall",
		"members":   []interface{}{"a@x.com", "b@x.com"},
		"endpoints": []interface{}{"ep1"},
	})
	d.SetId("t-1")

	if diags := ResourceAdaptiveTeamRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := stateStringSlice(d, "members"); got[0] != "a@x.com" || got[1] != "b@x.com" {
		t.Errorf("equal member sets should keep the configured ordering, got %v", got)
	}
	// The endpoint really did change server-side, so that one is refreshed.
	if got := stateStringSlice(d, "endpoints"); len(got) != 1 || got[0] != "ep2" {
		t.Errorf("changed endpoints should be refreshed, got %v", got)
	}
	if got := d.Get("slack_channel_id").(string); got != "" {
		t.Errorf("a slack channel the configuration never set must not be adopted, got %q", got)
	}
}

// A channel removed server-side is drift on a configuration that set one, so
// it has to reach state rather than being left stale.
func TestTeamReadReportsRemovedSlackChannel(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t-1","name":"oncall","members":["a@x.com"],"slackChannelId":""}`))
	})

	res := ResourceAdaptiveTeam()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":             "oncall",
		"members":          []interface{}{"a@x.com"},
		"slack_channel_id": "C123",
	})
	d.SetId("t-1")

	if diags := ResourceAdaptiveTeamRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("slack_channel_id").(string); got != "" {
		t.Errorf("a removed slack channel should be reflected in state, got %q", got)
	}
}

func TestTeamReadPopulatesSlackOnImport(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t-1","name":"oncall","members":["a@x.com"],"slackChannelId":"C999"}`))
	})

	res := ResourceAdaptiveTeam()
	// Import seeds state with nothing but the ID.
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})
	d.SetId("t-1")

	if diags := ResourceAdaptiveTeamRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("name").(string); got != "oncall" {
		t.Errorf("name: got %q", got)
	}
	if got := d.Get("slack_channel_id").(string); got != "C999" {
		t.Errorf("import should adopt the server's slack channel, got %q", got)
	}
}

func TestTeamReadDropsDeleted(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, notFound)

	res := ResourceAdaptiveTeam()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "gone"})
	d.SetId("t-1")

	if diags := ResourceAdaptiveTeamRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("a deleted group must not fail the refresh: %+v", diags)
	}
	if d.Id() != "" {
		t.Errorf("id should be cleared, got %q", d.Id())
	}
}

// ---------------------------------------------------------------------------
// Script
// ---------------------------------------------------------------------------

func scriptReadBody(digest string) string {
	return `{"id":"sc-1","name":"backup","endpoint":"ep1","description":"nightly",
		"parameterDescriptions":{"path":"where to write"},"commandOmitted":true,
		"commandDigest":"` + digest + `"}`
}

// The command is write-only. A server fingerprint that no longer matches the
// recorded one is the only evidence of an out-of-band edit, and clearing the
// command is what makes the next plan show it being rewritten.
func TestScriptReadClearsCommandOnDigestDrift(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(scriptReadBody("sha256:new")))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":           "backup",
		"command":        "pg_dump",
		"endpoint":       "ep1",
		"command_digest": "sha256:old",
	})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("command").(string); got != "" {
		t.Errorf("command should be cleared on digest drift, got %q", got)
	}
	if got := d.Get("command_digest").(string); got != "sha256:new" {
		t.Errorf("command_digest should be refreshed, got %q", got)
	}
}

func TestScriptReadKeepsCommandWhenDigestMatches(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(scriptReadBody("sha256:same")))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":           "backup",
		"command":        "pg_dump",
		"endpoint":       "ep1",
		"description":    "stale",
		"command_digest": "sha256:same",
	})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("command").(string); got != "pg_dump" {
		t.Errorf("command must survive a matching digest, got %q", got)
	}
	// description is a configured attribute here, so a server-side change to it
	// is real drift and does reach state.
	if got := d.Get("description").(string); got != "nightly" {
		t.Errorf("description: got %q", got)
	}
}

// A description the configuration never set must not be adopted from the
// server: that would show a diff on every plan.
func TestScriptReadIgnoresUnsetOptionalDescription(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(scriptReadBody("sha256:same")))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":     "backup",
		"command":  "pg_dump",
		"endpoint": "ep1",
	})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("description").(string); got != "" {
		t.Errorf("unset description should stay unset, got %q", got)
	}
	if got := stateStringMap(d, "parameter_descriptions"); len(got) != 0 {
		t.Errorf("unset parameter_descriptions should stay unset, got %v", got)
	}
}

// A server that predates the digest field returns nothing to compare against;
// the command must be left alone rather than cleared on every refresh.
func TestScriptReadKeepsCommandWithoutServerDigest(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sc-1","name":"backup","endpoint":"ep1"}`))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":           "backup",
		"command":        "pg_dump",
		"endpoint":       "ep1",
		"command_digest": "sha256:old",
	})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if got := d.Get("command").(string); got != "pg_dump" {
		t.Errorf("command must survive a server that reports no digest, got %q", got)
	}
}

// Import can never recover the script body, so it warns instead of silently
// leaving an empty required attribute.
func TestScriptReadWarnsOnImport(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(scriptReadBody("sha256:new")))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})
	d.SetId("sc-1")

	diags := ResourceAdaptiveScriptRead(context.Background(), d, client)
	if diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	if len(diags) != 1 || !strings.Contains(diags[0].Detail, "does not return script bodies") {
		t.Fatalf("expected a warning about the missing command, got %+v", diags)
	}
	if got := d.Get("command").(string); got != "" {
		t.Errorf("import must not invent a command, got %q", got)
	}
	if got := d.Get("name").(string); got != "backup" {
		t.Errorf("import should populate name, got %q", got)
	}
	if got := stateStringMap(d, "parameter_descriptions"); got["path"] != "where to write" {
		t.Errorf("import should populate parameter_descriptions, got %v", got)
	}
}

func TestScriptReadDropsDeleted(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, notFound)

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "gone"})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("a deleted script must not fail the refresh: %+v", diags)
	}
	if d.Id() != "" {
		t.Errorf("id should be cleared, got %q", d.Id())
	}
}

// Create must send the whole request — including the fields that were dropped
// before — and record the server's fingerprint for later drift detection.
func TestScriptCreateSendsFullRequestAndStoresDigest(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/read/") {
			_, _ = w.Write([]byte(scriptReadBody("sha256:fresh")))
			return
		}
		_, _ = w.Write([]byte(`{"id":"sc-1"}`))
	})

	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":        "backup",
		"command":     "pg_dump",
		"endpoint":    "ep1",
		"description": "nightly",
		"parameter_descriptions": map[string]interface{}{
			"path": "where to write",
		},
	})

	if diags := ResourceAdaptiveScriptCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create: %+v", diags)
	}
	if d.Id() != "sc-1" {
		t.Fatalf("id: got %q", d.Id())
	}
	body := api.body[0]
	if body["Description"] != "nightly" {
		t.Errorf("Description not sent: %v", body)
	}
	params, ok := body["ParameterDescriptions"].(map[string]any)
	if !ok || params["path"] != "where to write" {
		t.Errorf("ParameterDescriptions not sent: %v", body)
	}
	if got := d.Get("command_digest").(string); got != "sha256:fresh" {
		t.Errorf("command_digest should be captured after create, got %q", got)
	}
}
