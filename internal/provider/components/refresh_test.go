package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"gopkg.in/yaml.v2"
)

func TestRefreshCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, route, body string
		resource          *schema.Resource
		initial, want     map[string]interface{}
	}{
		{"legacy authorization", "authorization", `{"Name":"renamed","Description":"changed","Permission":"SELECT","Status":"created"}`, ResourceAdaptiveAuthorization(),
			map[string]interface{}{"name": "old", "resource_type": "postgres", "permissions": "old"}, map[string]interface{}{"name": "renamed", "description": "changed", "permissions": "SELECT", "resource_type": "postgres"}},
		{"canonical authorization", "authorization", `{"name":"new","resourceType":"mysql","permissions":"GRANT","status":"created"}`, ResourceAdaptiveAuthorization(),
			map[string]interface{}{"name": "old", "resource_type": "postgres", "permissions": "old"}, map[string]interface{}{"name": "new", "resource_type": "mysql", "permissions": "GRANT"}},
		{"legacy endpoint keeps omitted fields", "session", `{"Name":"ep","Resource":"db","SessionType":"cli","Status":"paused","SessionUsers":["b","a"],"IsJITEnabled":false,"AccessApprovers":[]}`, ResourceAdaptiveSession(),
			map[string]interface{}{"name": "ep", "resource": "db", "type": "direct", "users": []interface{}{"a", "b"}, "is_jit_enabled": true, "jit_approvers": []interface{}{"admin"}, "idle_timeout": "1h", "script_only_access": true},
			map[string]interface{}{"type": "direct", "users": []interface{}{"a", "b"}, "is_jit_enabled": false, "jit_approvers": []interface{}{}, "idle_timeout": "1h", "script_only_access": true}},
		{"current endpoint preserves defaults and aliases", "session", `{"name":"ep","resource":"db","sessionType":"cli","cluster":"default-cluster","ttl":"2160h","cpu":"250m","memory":"256Mi","idleTimeout":"60m","scriptOnlyAccess":false,"disableWebCli":true,"autoApproval":false,"jitTotalApprovers":0}`, ResourceAdaptiveSession(),
			map[string]interface{}{"name": "ep", "resource": "db", "type": "direct", "idle_timeout": "1h", "script_only_access": true, "authorization": "removed", "max_jit_request_duration": "2h"},
			map[string]interface{}{"type": "direct", "cluster": "", "ttl": "", "cpu": "", "memory": "", "idle_timeout": "1h", "script_only_access": false, "disable_web_cli": true, "auto_approval": false, "jit_total_approvers": 0, "authorization": "", "max_jit_request_duration": ""}},
		{"team canonical and legacy ordering", "team", `{"Name":"renamed","Members":["b","a"],"Endpoints":[]}`, ResourceAdaptiveTeam(),
			map[string]interface{}{"name": "old", "members": []interface{}{"a", "b"}, "endpoints": []interface{}{"old"}}, map[string]interface{}{"name": "renamed", "members": []interface{}{"a", "b"}, "endpoints": []interface{}{}}},
		{"script body remains write only", "script", `{"id":"s","name":"new","endpoint":"ep","commandOmitted":true,"command":"<REDACTED>","commandDigest":"opaque","helpText":""}`, ResourceAdaptiveScript(),
			map[string]interface{}{"name": "old", "endpoint": "ep", "command": "echo private", "help_text": "old"}, map[string]interface{}{"name": "new", "command": "echo private", "command_digest": "opaque", "help_text": ""}},
		{"resource public fields and secrets", "resource", `{"id":"r","name":"db","integrationType":"postgres","userTags":["b=2","a=1"],"configuration":{"hostname":"new-host","port":"5433","password":"<REDACTED>","username":"new-user"},"redactedKeys":["password"],"redactedDigests":{"password":"digest"}}`, ResourceAdaptiveResource(),
			map[string]interface{}{"name": "db", "type": "postgres", "host": "old-host", "port": "5432", "username": "old-user", "password": "private", "tags": []interface{}{"a=1", "b=2"}},
			map[string]interface{}{"host": "new-host", "port": "5433", "username": "new-user", "password": "private", "tags": []interface{}{"a=1", "b=2"}}},
		{"resource nested redaction preserves blob", "resource", `{"name":"services","integrationType":"servicelist","configuration":{"urls":"partial"},"redactedKeys":["urls[0].password"]}`, ResourceAdaptiveResource(),
			map[string]interface{}{"name": "services", "type": "services", "urls": "original with password"}, map[string]interface{}{"type": "services", "urls": "original with password"}},
		{"legacy schedule does not erase pattern", "schedule", `{"id":"s","name":"schedule","scheduleType":"custom","isActive":true}`, ResourceAdaptiveSchedule(),
			map[string]interface{}{"name": "schedule", "schedule_type": "custom", "all_day": true, "description": "keep", "start_hour": 9, "end_hour": 17, "users": []interface{}{"a"}, "weekdays": []interface{}{"Monday"}},
			map[string]interface{}{"all_day": true, "description": "keep", "start_hour": 9, "end_hour": 17, "users": []interface{}{"a"}, "weekdays": []interface{}{"Monday"}}},
		{"full schedule clears removed values", "schedule", `{"id":"s","name":"schedule","scheduleType":"custom","isActive":false,"allDay":false,"startHour":8,"startMinute":0,"endHour":18,"endMinute":0,"weekdays":["tuesday","monday"],"users":[],"teams":[],"endpoints":["ep"]}`, ResourceAdaptiveSchedule(),
			map[string]interface{}{"name": "schedule", "schedule_type": "custom", "description": "remove", "timezone": "Europe/Amsterdam", "expires_at": "2030-01-01T00:00:00Z", "max_access_time": 60, "users": []interface{}{"old"}, "weekdays": []interface{}{"Monday", "Tuesday"}},
			map[string]interface{}{"is_active": false, "description": "", "timezone": "", "expires_at": "", "max_access_time": 0, "start_hour": 8, "end_hour": 18, "users": []interface{}{}, "endpoints": []interface{}{"ep"}, "weekdays": []interface{}{"Monday", "Tuesday"}}},
		{"all day normalization", "schedule", `{"name":"s","scheduleType":"everyday","allDay":true,"startHour":0,"startMinute":0,"endHour":23,"endMinute":59}`, ResourceAdaptiveSchedule(),
			map[string]interface{}{"name": "s", "schedule_type": "everyday", "all_day": true}, map[string]interface{}{"all_day": true, "end_hour": 0, "end_minute": 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v1/terraform/"+tc.route+"/read/id" {
					t.Errorf("wrong read URL: %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			d := schema.TestResourceDataRaw(t, tc.resource.Schema, tc.initial)
			d.SetId("id")
			if diags := tc.resource.ReadContext(context.Background(), d, adaptive.NewClient("token", srv.URL)); diags.HasError() {
				t.Fatal(diags)
			}
			if calls != 1 {
				t.Fatalf("made %d read calls", calls)
			}
			for key, want := range tc.want {
				if got := d.Get(key); !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %#v, want %#v", key, got, want)
				}
			}
		})
	}
}

func TestRefreshFailurePreservesState(t *testing.T) {
	for name, resource := range map[string]*schema.Resource{
		"authorization": ResourceAdaptiveAuthorization(), "resource": ResourceAdaptiveResource(), "session": ResourceAdaptiveSession(),
		"script": ResourceAdaptiveScript(), "team": ResourceAdaptiveTeam(), "schedule": ResourceAdaptiveSchedule(), "dataprotection": ResourceAdaptiveDataProtection(),
	} {
		for _, tc := range []struct {
			name, body string
			status     int
			keep, fail bool
		}{
			{"deleted", `"not found"`, 404, false, false},
			{"unsupported", "404 page not found", 404, true, false},
			{"forbidden", `"not allowed"`, 403, true, true},
			{"server error", `"could not read"`, 500, true, true},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
				}))
				defer srv.Close()
				d := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{})
				d.SetId("keep-me")
				diags := resource.ReadContext(context.Background(), d, adaptive.NewClient("token", srv.URL))
				if (d.Id() != "") != tc.keep || diags.HasError() != tc.fail {
					t.Fatalf("id=%q diagnostics=%v", d.Id(), diags)
				}
			})
		}
	}
}

func TestPolicySchemaDistinguishesOmittedAndExplicitClears(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		input := map[string]interface{}{"name": "ep", "resource": "db"}
		if explicit {
			input["auto_approval"] = false
			input["jit_multi_approver"] = false
			input["jit_total_approvers"] = 0
			input["max_jit_request_duration"] = ""
		}
		d := resourceDataWithConfig(t, ResourceAdaptiveSession(), input)
		policy := sessionPolicyFromSchema(d)
		if (policy.AutoApproval != nil) != explicit || (policy.JITMultiApprover != nil) != explicit || (policy.JITTotalApprovers != nil) != explicit || (policy.MaxJITRequestDuration != nil) != explicit {
			t.Fatalf("explicit=%v: %#v", explicit, policy)
		}
	}
}

func TestResourceRefreshMappingsMatchWriteContract(t *testing.T) {
	resource := ResourceAdaptiveResource()
	for integration, fields := range resourceConfigurationFields {
		if len(fields) == 0 {
			continue
		}
		t.Run(integration, func(t *testing.T) {
			input := map[string]interface{}{"name": "test", "type": integration}
			for _, attribute := range fields {
				field, ok := resource.Schema[attribute]
				if !ok {
					t.Fatalf("unknown attribute %s", attribute)
				}
				switch field.Type {
				case schema.TypeString:
					input[attribute] = "value-for-" + attribute
				case schema.TypeBool:
					input[attribute] = true
				case schema.TypeList:
					input[attribute] = []interface{}{"host-a", "host-b"}
				default:
					t.Fatalf("unhandled type for %s", attribute)
				}
			}
			// The old Cockroach converter reads this guard before tls_root_cert.
			if integration == "cockroachdb" {
				input["root_cert"] = "guard"
			}
			d := schema.TestResourceDataRaw(t, resource.Schema, input)
			configuration, err := schemaToResourceIntegrationConfiguration(d, integration)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := yaml.Marshal(configuration)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]interface{}
			if err := yaml.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			for key, attribute := range fields {
				want := d.Get(attribute)
				if attribute == "hosts" {
					want = "host-a\nhost-b"
				}
				if got := wire[key]; !reflect.DeepEqual(got, want) {
					t.Errorf("%s -> %s: write=%#v, state=%#v", key, attribute, got, want)
				}
			}
		})
	}
}

func TestScriptMetadataWriteAndDigestChange(t *testing.T) {
	var body map[string]interface{}
	digest := "first"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"id":"s"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"s","name":"script","endpoint":"ep","commandOmitted":true,"commandDigest":"` + digest + `"}`))
	}))
	defer srv.Close()
	r := ResourceAdaptiveScript()
	d := resourceDataWithConfig(t, r, map[string]interface{}{"name": "script", "endpoint": "ep", "command": "echo private", "help_text": "", "description": "test", "parameter_descriptions": map[string]interface{}{"param": "help"}})
	c := adaptive.NewClient("token", srv.URL)
	if diags := r.CreateContext(context.Background(), d, c); diags.HasError() {
		t.Fatal(diags)
	}
	if body["helpText"] != "" || body["Description"] != "test" || body["Command"] != "echo private" {
		t.Fatalf("metadata lost: %v", body)
	}
	if diags := r.ReadContext(context.Background(), d, c); len(diags) != 0 {
		t.Fatal(diags)
	}
	digest = "changed"
	if diags := r.ReadContext(context.Background(), d, c); len(diags) != 1 || diags.HasError() || !strings.Contains(diags[0].Summary, "changed") {
		t.Fatal(diags)
	}
	if d.Get("command") != "echo private" {
		t.Fatal("command was overwritten")
	}
}

func resourceDataWithConfig(t *testing.T, r *schema.Resource, input map[string]interface{}) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, r.Schema, input)
	d.SetId("test-config")
	state := d.State()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var config ctyjson.SimpleJSONValue
	if err := json.Unmarshal(encoded, &config); err != nil {
		t.Fatal(err)
	}
	state.RawConfig = config.Value
	result, err := schema.InternalMap(r.Schema).Data(state, nil)
	if err != nil {
		t.Fatal(err)
	}
	result.SetId("")
	return result
}

func TestLegacyScriptStateDoesNotPlanMetadataWrite(t *testing.T) {
	for _, body := range []string{`{"name":"script","endpoint":"ep","commandOmitted":true}`, `{"Name":"script","Endpoint":"ep"}`, "404 page not found"} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body == "404 page not found" {
					w.WriteHeader(404)
				}
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			r := ResourceAdaptiveScript()
			// Exactly the state attributes supported before metadata was added.
			state := &terraform.InstanceState{ID: "s", Attributes: map[string]string{"id": "s", "name": "script", "endpoint": "ep", "command": "echo private"}}
			client := adaptive.NewClient("token", srv.URL)
			state, diags := r.RefreshWithoutUpgrade(context.Background(), state, client)
			if diags.HasError() {
				t.Fatal(diags)
			}
			config := terraform.NewResourceConfigRaw(map[string]interface{}{"name": "script", "endpoint": "ep", "command": "echo private"})
			diff, err := r.SimpleDiff(context.Background(), state, config, client)
			if err != nil {
				t.Fatal(err)
			}
			if change := diff.Attributes["parameter_descriptions.%"]; change != nil {
				t.Fatalf("upgrade plans an unrequested metadata write: %#v", change)
			}
		})
	}
}

func TestLegacyComputedCollectionsDoNotPlanWrites(t *testing.T) {
	for _, tc := range []struct {
		name       string
		resource   *schema.Resource
		config     map[string]interface{}
		collection string
	}{
		{"endpoint", ResourceAdaptiveSession(), map[string]interface{}{"name": "ep", "resource": "db"}, "groups.#"},
		{"resource", ResourceAdaptiveResource(), map[string]interface{}{"name": "db", "type": "postgres"}, "redacted_digests.%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(202)
				_, _ = w.Write([]byte(`{"Status":"created"}`))
			}))
			defer srv.Close()
			d := schema.TestResourceDataRaw(t, tc.resource.Schema, tc.config)
			d.SetId("id")
			state := d.State()
			delete(state.Attributes, tc.collection)
			client := adaptive.NewClient("token", srv.URL)
			state, diags := tc.resource.RefreshWithoutUpgrade(context.Background(), state, client)
			if diags.HasError() {
				t.Fatal(diags)
			}
			diff, err := tc.resource.SimpleDiff(context.Background(), state, terraform.NewResourceConfigRaw(tc.config), client)
			if err != nil {
				t.Fatal(err)
			}
			if change := diff.Attributes[tc.collection]; change != nil {
				t.Fatalf("upgrade plans a collection write: %#v", change)
			}
		})
	}
}

func TestBareEndpointTagsKeepConfiguredSpelling(t *testing.T) {
	r := ResourceAdaptiveSession()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "ep", "resource": "db", "tags": []interface{}{"production", "flags=one=two"}})
	data := map[string]interface{}{"userTags": []interface{}{"flags=one=two", "production="}}
	if diags := setResponseFields(d, data, map[string][]string{"tags": {"userTags"}}); diags.HasError() {
		t.Fatal(diags)
	}
	if !reflect.DeepEqual(d.Get("tags"), []interface{}{"production", "flags=one=two"}) {
		t.Fatalf("tags changed: %v", d.Get("tags"))
	}
}
