package components

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// jsonServer serves body for any request whose path contains wantPath.
func jsonServer(t *testing.T, wantPath, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, wantPath) {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// A 404 must clear the id so Terraform plans a recreate rather than failing the
// refresh. Before this, every Read either did nothing or returned an error here.
func TestReadsDropDeletedObjectsFromState(t *testing.T) {
	cases := []struct {
		name string
		path string
		res  func() *schema.Resource
		read schema.ReadContextFunc
		raw  map[string]interface{}
	}{
		{
			name: "endpoint",
			path: "/terraform/session/read/",
			res:  ResourceAdaptiveSession,
			read: ResourceAdaptiveSessionRead,
			raw:  map[string]interface{}{"name": "ep", "resource": "pg"},
		},
		{
			name: "resource",
			path: "/terraform/resource/read/",
			res:  ResourceAdaptiveResource,
			read: ResourceAdaptiveResourceRead,
			raw:  map[string]interface{}{"name": "pg", "type": "postgres"},
		},
		{
			name: "group",
			path: "/terraform/team/read/",
			res:  ResourceAdaptiveTeam,
			read: ResourceAdaptiveTeamRead,
			raw:  map[string]interface{}{"name": "team"},
		},
		{
			name: "script",
			path: "/terraform/script/read/",
			res:  ResourceAdaptiveScript,
			read: ResourceAdaptiveScriptRead,
			raw:  map[string]interface{}{"name": "s", "command": "ls", "endpoint": "ep"},
		},
		{
			name: "authorization",
			path: "/terraform/authorization/read/",
			res:  ResourceAdaptiveAuthorization,
			read: ResourceAdaptiveAuthorizationRead,
			raw:  map[string]interface{}{"name": "a", "resource_type": "postgres", "permissions": "x"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := jsonServer(t, tc.path, `"not found"`, http.StatusNotFound)
			defer srv.Close()

			client := adaptive.NewClient("test-token", srv.URL)
			d := schema.TestResourceDataRaw(t, tc.res().Schema, tc.raw)
			d.SetId("gone-1")

			if diags := tc.read(context.Background(), d, client); diags.HasError() {
				t.Fatalf("read of a deleted %s returned diagnostics: %+v", tc.name, diags)
			}
			if d.Id() != "" {
				t.Errorf("expected id to be cleared for a deleted %s, got %q", tc.name, d.Id())
			}
		})
	}
}

// The endpoint read must refresh the fields that were writable but unreadable
// before, and must leave `type` alone: the backend stores "direct" as "cli", so
// writing sessionType back would show drift on every endpoint declared "direct".
func TestResourceAdaptiveSessionRead_RefreshesAllFields(t *testing.T) {
	srv := jsonServer(t, "/terraform/session/read/", `{
		"id": "ep-1",
		"name": "prod-db",
		"resource": "pg-main",
		"cluster": "cluster-a",
		"authorization": "readonly",
		"status": "created",
		"sessionType": "cli",
		"ttl": "7d",
		"idleTimeout": "30m",
		"pauseTimeout": "1h",
		"memory": "512Mi",
		"cpu": "0.5",
		"isJitEnabled": true,
		"scriptOnlyAccess": true,
		"sessionUsers": ["a@example.com"],
		"groups": ["team-a"],
		"accessApprovers": ["b@example.com"],
		"userTags": ["env=prod"]
	}`, http.StatusAccepted)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{
		"name":     "prod-db",
		"resource": "pg-main",
		"type":     "direct",
	})
	d.SetId("ep-1")

	if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	for attr, want := range map[string]interface{}{
		"name":               "prod-db",
		"resource":           "pg-main",
		"cluster":            "cluster-a",
		"authorization":      "readonly",
		"ttl":                "7d",
		"idle_timeout":       "30m",
		"pause_timeout":      "1h",
		"memory":             "512Mi",
		"cpu":                "0.5",
		"is_jit_enabled":     true,
		"script_only_access": true,
	} {
		if got := d.Get(attr); got != want {
			t.Errorf("%s = %v, want %v", attr, got, want)
		}
	}

	// `type` must survive untouched, still holding the config's spelling.
	if got := d.Get("type").(string); got != "direct" {
		t.Errorf(`type was overwritten from sessionType: got %q, want "direct"`, got)
	}

	users := d.Get("users").([]interface{})
	if len(users) != 1 || users[0] != "a@example.com" {
		t.Errorf("users not refreshed: %#v", users)
	}
	groups := d.Get("groups").([]interface{})
	if len(groups) != 1 || groups[0] != "team-a" {
		t.Errorf("groups not refreshed: %#v", groups)
	}
}

// Members and endpoints come back as emails and names, which is what the write
// path accepts, so they must land in state verbatim.
func TestResourceAdaptiveTeamRead_RefreshesMembersAndEndpoints(t *testing.T) {
	srv := jsonServer(t, "/terraform/team/read/", `{
		"id": "team-1",
		"name": "platform",
		"members": ["a@example.com", "b@example.com"],
		"endpoints": ["prod-db", "stage-db"]
	}`, http.StatusOK)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveTeam().Schema, map[string]interface{}{"name": "platform"})
	d.SetId("team-1")

	if diags := ResourceAdaptiveTeamRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	members := d.Get("members").([]interface{})
	if len(members) != 2 || members[0] != "a@example.com" {
		t.Errorf("members not refreshed: %#v", members)
	}
	endpoints := d.Get("endpoints").([]interface{})
	if len(endpoints) != 2 || endpoints[1] != "stage-db" {
		t.Errorf("endpoints not refreshed: %#v", endpoints)
	}
}

// A script body can embed credentials, so the backend withholds it. Read must
// leave the prior command in place rather than blanking it, which would show as
// a permanent diff.
func TestResourceAdaptiveScriptRead_LeavesCommandAlone(t *testing.T) {
	srv := jsonServer(t, "/terraform/script/read/", `{
		"id": "sc-1",
		"name": "renamed-upstream",
		"endpoint": "prod-db",
		"commandOmitted": true
	}`, http.StatusOK)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveScript().Schema, map[string]interface{}{
		"name":     "original",
		"command":  "psql -c 'select 1'",
		"endpoint": "prod-db",
	})
	d.SetId("sc-1")

	if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	if got := d.Get("name").(string); got != "renamed-upstream" {
		t.Errorf("name not refreshed: got %q", got)
	}
	if got := d.Get("command").(string); got != "psql -c 'select 1'" {
		t.Errorf("command must keep its prior value, got %q", got)
	}
}

// resource_type was commented out of the backend response, so drift on it was
// invisible; it now round-trips.
func TestResourceAdaptiveAuthorizationRead_RefreshesResourceType(t *testing.T) {
	srv := jsonServer(t, "/terraform/authorization/read/", `{
		"id": "auth-1",
		"name": "readonly",
		"description": "read only access",
		"resourceType": "postgres",
		"permissions": "grants: select"
	}`, http.StatusOK)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveAuthorization().Schema, map[string]interface{}{
		"name":          "readonly",
		"resource_type": "mysql",
		"permissions":   "grants: select",
	})
	d.SetId("auth-1")

	if diags := ResourceAdaptiveAuthorizationRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	if got := d.Get("resource_type").(string); got != "postgres" {
		t.Errorf("resource_type not refreshed: got %q want postgres", got)
	}
	if got := d.Get("description").(string); got != "read only access" {
		t.Errorf("description not refreshed: got %q", got)
	}
}

// A key listed in redactedKeys must not be written into state. The backend
// already strips the value, so setting it would clear a live credential.
func TestResourceAdaptiveResourceRead_SkipsRedactedAttributes(t *testing.T) {
	srv := jsonServer(t, "/terraform/resource/read/", `{
		"id": "res-1",
		"name": "pg-main",
		"integrationType": "postgres",
		"userTags": ["env=prod"],
		"configuration": {"hostname": "db.internal", "port": 5432, "username": "svc"},
		"redactedKeys": ["password"]
	}`, http.StatusOK)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "pg-main",
		"type":     "postgres",
		"password": "still-secret",
	})
	d.SetId("res-1")

	if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	if got := d.Get("host").(string); got != "db.internal" {
		t.Errorf("host not refreshed from hostname: got %q", got)
	}
	// port is a string in the schema but decodes as a number.
	if got := d.Get("port").(string); got != "5432" {
		t.Errorf("port not normalised to string: got %q", got)
	}
	if got := d.Get("username").(string); got != "svc" {
		t.Errorf("username not refreshed: got %q", got)
	}
	if got := d.Get("password").(string); got != "still-secret" {
		t.Errorf("redacted attribute was overwritten: got %q", got)
	}
}

// "services" is stored as "servicelist"; mapping it back keeps a config that
// declares type = "services" from showing drift forever.
func TestResourceAdaptiveResourceRead_MapsServicelistBackToServices(t *testing.T) {
	srv := jsonServer(t, "/terraform/resource/read/", `{
		"id": "res-2",
		"name": "svc",
		"integrationType": "servicelist",
		"configuration": {"urls": "https://a.example.com"}
	}`, http.StatusOK)
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "svc",
		"type": "services",
	})
	d.SetId("res-2")

	if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	if got := d.Get("type").(string); got != "services" {
		t.Errorf(`type = %q, want "services"`, got)
	}
}

// The overloaded keys are the ones worth pinning: writing a value into the wrong
// attribute would not fail loudly, it would just show drift forever.
func TestResourceConfigAttrOverloadedKeys(t *testing.T) {
	cases := []struct {
		integrationType string
		key             string
		wantAttr        string
		wantOK          bool
	}{
		{"postgres", "hostname", "host", true},
		{"mysql", "hostname", "host", true},
		{"paloalto_ngfw", "hostname", "hostname", true},
		{"syslog", "hostname", "hostname", true},
		{"snowflake", "databaseAccount", "hostname", true},
		{"snowflake", "databaseUsername", "username", true},
		{"splunk", "url", "url", true},
		{"coralogix", "url", "uri", true},
		{"elasticsearch", "url", "uri", true},
		{"paloalto_ngfw", "login_url", "login_url", true},
		{"cisco_ngfw", "login_url", "uri", true},
		{"azurecosmosnosql", "endpoint", "uri", true},
		{"aws", "aws_region_name", "region_name", true},
		{"awssecretsmanager", "aws_region_name", "aws_region_name", true},
		// Keys the provider never wrote have no inverse and must be ignored.
		{"postgres", "version", "", false},
		{"postgres", "totally_unknown", "", false},
	}

	for _, tc := range cases {
		gotAttr, gotOK := resourceConfigAttr(tc.integrationType, tc.key)
		if gotOK != tc.wantOK || gotAttr != tc.wantAttr {
			t.Errorf("resourceConfigAttr(%q, %q) = (%q, %v), want (%q, %v)",
				tc.integrationType, tc.key, gotAttr, gotOK, tc.wantAttr, tc.wantOK)
		}
	}
}

// Every registered resource must be importable; Read is what hydrates state
// afterwards, so an importer without a working Read would produce empty state.
func TestAllResourcesAreImportable(t *testing.T) {
	for name, res := range map[string]*schema.Resource{
		"adaptive_endpoint":      ResourceAdaptiveSession(),
		"adaptive_resource":      ResourceAdaptiveResource(),
		"adaptive_authorization": ResourceAdaptiveAuthorization(),
		"adaptive_group":         ResourceAdaptiveTeam(),
		"adaptive_script":        ResourceAdaptiveScript(),
		"adaptive_schedule":      ResourceAdaptiveSchedule(),
	} {
		if res.Importer == nil || res.Importer.StateContext == nil {
			t.Errorf("%s has no importer", name)
		}
	}
}
