package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// This additive real-CLI suite covers the five non-resource/non-endpoint
// families. TestSecretTerraformEngine and TestEndpointTerraformEngine supply
// those two families' existing engine coverage. Fixtures below use backend wire
// keys, not provider DTOs, and model documented partial-update limitations.
func TestFamiliesTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	tf, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline plugin build: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"authorization", "group", "script", "schedule", "dataprotection"} {
		t.Run(family, func(t *testing.T) {
			h, api := newFamiliesEngine(t, tf, bin, family)
			address := "adaptive_" + family + ".test"
			config := func(stage int) { familiesEngineConfig(h, family, stage) }
			config(0)
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 0)
			familiesEngineState(h, address, "id", api.id)
			config(1)
			h.run(2, "plan", "-detailed-exitcode", "-out=update.tfplan")
			familiesEngineActions(h, address, "update.tfplan", "update")
			h.run(0, "apply", "-auto-approve", "update.tfplan")
			api.assert(t, 1)
			for i := 0; i < 2; i++ {
				h.run(0, "apply", "-refresh-only", "-auto-approve")
				h.run(0, "plan", "-detailed-exitcode")
			}
			// Clears with an authoritative backend-supported representation. Unsupported
			// scalar clears are separately rejected at plan below, not faked as fixed.
			config(2)
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 2)
			h.run(0, "plan", "-detailed-exitcode")
			if family == "schedule" {
				// Removing is_active restores Terraform's explicit true default,
				// rather than silently preserving the prior false remote value.
				familiesEngineConfigExtra(h, family, 2, "is_active = __omit__")
				h.run(0, "apply", "-auto-approve")
				api.mu.Lock()
				active := api.remote["isActive"]
				api.mu.Unlock()
				if active != true {
					t.Fatal("is_active omission did not reset to the documented true default")
				}
				h.run(0, "plan", "-detailed-exitcode")
				config(2)
				h.run(0, "apply", "-auto-approve")
				api.assert(t, 2)
			}
			if family == "script" {
				api.mu.Lock()
				api.remote["commandDigest"] = "externally-rotated"
				api.mu.Unlock()
				for i := 0; i < 2; i++ {
					h.run(0, "apply", "-refresh-only", "-auto-approve")
					familiesEngineState(h, address, "command", "")
				}
				h.run(2, "plan", "-detailed-exitcode", "-out=command.tfplan")
				familiesEngineActions(h, address, "command.tfplan", "update")
				h.run(0, "apply", "-auto-approve", "command.tfplan")
				api.assert(t, 2)
				h.run(0, "plan", "-detailed-exitcode")
			}
			// Import is an actual ID-only state transition; script command is withheld.
			familiesEngineForget(h, address)
			h.run(0, "import", address, api.id)
			familiesEngineState(h, address, "id", api.id)
			if family == "script" {
				familiesEngineState(h, address, "command", nil)
				h.run(0, "apply", "-auto-approve")
			}
			h.run(0, "plan", "-detailed-exitcode")
			familiesEngineForget(h, address)
			h.run(1, "import", address, "missing-id")
			h.run(0, "import", address, api.id)
			if family == "script" {
				h.run(0, "apply", "-auto-approve")
			}
			if family == "authorization" || family == "script" || family == "dataprotection" || family == "schedule" {
				before := api.mutationCount()
				extra := map[string]string{"authorization": "resource_type = \"postgres\"", "script": "endpoint = \"other\"", "dataprotection": "resource = \"other\"", "schedule": "timezone = \"Asia/Kolkata\""}[family]
				familiesEngineConfigExtra(h, family, 2, extra)
				out := h.run(1, "plan")
				if !strings.Contains(out, "cannot be updated") {
					t.Fatal("immutable plan failed for unrelated reason", engineSafe(out))
				}
				if api.mutationCount() != before {
					t.Fatal("rejected plan mutated backend")
				}
				config(2)
			}
			if family == "authorization" || family == "script" || family == "schedule" {
				before := api.mutationCount()
				field := "description"
				extra := "description = \"\""
				if family == "schedule" {
					field = "max_access_time"
					extra = "max_access_time = 0"
				}
				familiesEngineConfigExtra(h, family, 2, extra)
				out := h.run(1, "plan")
				if !strings.Contains(out, field+" cannot be cleared") {
					t.Fatal("backend-limited clear was not rejected precisely", engineSafe(out))
				}
				if api.mutationCount() != before {
					t.Fatal("clear rejection mutated backend")
				}
				config(2)
			}
			api.mu.Lock()
			api.exists = false
			api.mu.Unlock()
			h.run(2, "plan", "-detailed-exitcode", "-out=deleted.tfplan")
			familiesEngineActions(h, address, "deleted.tfplan", "create")
			h.run(0, "apply", "-auto-approve", "deleted.tfplan")
			api.assert(t, 2)
			h.run(0, "destroy", "-auto-approve")
			api.mu.Lock()
			defer api.mu.Unlock()
			if family == "dataprotection" {
				if api.remote["scoped"] != false || len(api.remote["masks"].([]any)) != 0 {
					t.Fatal("destroy did not turn masking off")
				}
			} else if api.exists {
				t.Fatal("destroy left object present")
			}
			if len(api.errors) > 0 {
				t.Fatal(api.errors)
			}
			t.Logf("actual CLI lifecycle complete: %s; creates=%d updates=%d deletes=%d", family, api.creates, api.updates, api.deletes)
		})
	}
	t.Run("group_partial_create_retries_update_without_duplicate", func(t *testing.T) {
		h, api := newFamiliesEngine(t, tf, bin, "group")
		api.failSlack = true
		familiesEngineConfig(h, "group", 0)
		out := h.run(0, "apply", "-auto-approve")
		if !strings.Contains(out, "Slack channel update needs retry") {
			t.Fatal("missing partial-create warning")
		}
		familiesEngineState(h, "adaptive_group.test", "id", api.id)
		api.mu.Lock()
		api.failSlack = false
		api.mu.Unlock()
		h.run(0, "apply", "-auto-approve")
		api.assert(t, 0)
		h.run(0, "plan", "-detailed-exitcode")
		api.mu.Lock()
		if api.creates != 1 {
			t.Error("retry duplicated group")
		}
		api.mu.Unlock()
		h.run(0, "destroy", "-auto-approve")
	})
}

func newFamiliesEngine(t *testing.T, tf, bin, family string) (*secretEngine, *familiesEngineAPI) {
	h := newSecretEngine(t, tf, bin, false) // Reuse the sanitized HOME/env + CLI runner.
	route := family
	if family == "group" {
		route = "team"
	}
	api := &familiesEngineAPI{family: family, base: "/api/v1/terraform/" + route + "/", id: "family-" + family, remote: map[string]any{}}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	h.url = srv.URL
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) > 0 {
			t.Errorf("strict fake contract violations: %v", api.errors)
		}
	})
	return h, api
}

func familiesEngineConfig(h *secretEngine, family string, stage int) {
	familiesEngineConfigExtra(h, family, stage, "")
}
func familiesEngineConfigExtra(h *secretEngine, family string, stage int, extra string) {
	h.t.Helper()
	attrs := map[string]string{}
	attrs["name"] = `"family-test"`
	attrs["description"] = `"initial"`
	if stage > 0 {
		attrs["description"] = `"updated"`
	}
	switch family {
	case "authorization":
		attrs["resource_type"] = `"mysql"`
		attrs["permissions"] = `"SELECT"`
		if stage > 0 {
			attrs["permissions"] = `"UPDATE"`
		}
	case "group":
		delete(attrs, "description")
		attrs["members"] = `["member@example.invalid"]`
		attrs["endpoints"] = `["db"]`
		attrs["slack_channel_id"] = `"C123"`
		if stage > 0 {
			attrs["members"] = `["other@example.invalid"]`
		}
		if stage == 2 {
			attrs["members"] = `[]`
			attrs["endpoints"] = `[]`
			attrs["slack_channel_id"] = `""`
		}
	case "script":
		attrs["endpoint"] = `"db"`
		attrs["command"] = `"echo initial"`
		attrs["parameter_descriptions"] = `{path="initial"}`
		if stage > 0 {
			attrs["command"] = `"echo updated"`
			attrs["parameter_descriptions"] = `{path="updated"}`
		}
		if stage == 2 {
			attrs["parameter_descriptions"] = `{}`
		}
	case "schedule":
		attrs["schedule_type"] = `"custom"`
		attrs["timezone"] = `"UTC"`
		attrs["weekdays"] = `["monday"]`
		attrs["start_hour"] = "9"
		attrs["end_hour"] = "17"
		attrs["users"] = `["member@example.invalid"]`
		attrs["teams"] = `["ops"]`
		attrs["endpoints"] = `["db"]`
		attrs["expires_at"] = `"2030-01-01T00:00:00Z"`
		attrs["max_access_time"] = "60"
		if stage > 0 {
			attrs["start_hour"] = "10"
		}
		if stage == 2 {
			attrs["description"] = `""`
			attrs["expires_at"] = `""`
			attrs["users"] = `[]`
			attrs["teams"] = `[]`
			attrs["endpoints"] = `[]`
			attrs["start_hour"] = "0"
			attrs["end_hour"] = "0"
			attrs["all_day"] = "true"
			attrs["is_active"] = "false"
			attrs["max_access_time"] = "30"
		}
	case "dataprotection":
		attrs = map[string]string{"resource": `"db"`, "scoped": "true"}
		if stage > 0 {
			attrs["scoped"] = "false"
		}
		if stage == 2 {
			delete(attrs, "masks")
		}
	}
	if extra != "" {
		parts := strings.SplitN(extra, "=", 2)
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if value == "__omit__" {
			delete(attrs, key)
		} else {
			attrs[key] = value
		}
	}
	body := ""
	if family == "dataprotection" && stage < 2 {
		body = `masks {
 database_name = "app"
 tables {
 table_name = "people"
 masked = true
 masked_columns {
 column_name = "email"
 masking_type = "email"
 }
 }
 }
`
	}
	for k, v := range attrs {
		body += k + " = " + v + "\n"
	}
	engineWrite(h.t, filepath.Join(h.dir, "main.tf"), fmt.Sprintf("terraform {\n required_providers {\n adaptive = {source=\"adaptive-scale/adaptive\"}\n }\n}\nprovider \"adaptive\" {\n service_token=%q\n workspace_url=%q\n}\nresource \"adaptive_%s\" \"test\" {\n%s}\n", engineToken, h.url, family, body))
}
func familiesEngineForget(h *secretEngine, address string) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.terraform, "state", "rm", "-no-color", address)
	cmd.Dir = h.dir
	cmd.Env = h.env
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("state rm: %v %s", err, engineSafe(string(out)))
	}
}
func familiesEngineActions(h *secretEngine, address, file, action string) {
	var plan struct {
		Changes []struct {
			Address string `json:"address"`
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal([]byte(h.run(0, "show", "-json", file)), &plan); err != nil {
		h.t.Fatal(err)
	}
	for _, c := range plan.Changes {
		if c.Address == address && reflect.DeepEqual(c.Change.Actions, []string{action}) {
			return
		}
	}
	h.t.Fatalf("actual saved plan did not contain %s %s", action, address)
}
func familiesEngineState(h *secretEngine, address, key string, want any) {
	var state struct {
		Values struct {
			Root struct {
				Resources []struct {
					Address string         `json:"address"`
					Values  map[string]any `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal([]byte(h.run(0, "show", "-json")), &state); err != nil {
		h.t.Fatal(err)
	}
	for _, r := range state.Values.Root.Resources {
		if r.Address == address {
			if !reflect.DeepEqual(r.Values[key], want) {
				h.t.Fatalf("%s state %s differs (values withheld)", address, key)
			}
			return
		}
	}
	h.t.Fatal("missing actual engine state", address)
}

type familiesEngineAPI struct {
	mu                        sync.Mutex
	family, base, id          string
	exists, failSlack         bool
	remote                    map[string]any
	errors                    []string
	creates, updates, deletes int
}

func (a *familiesEngineAPI) mutationCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.creates + a.updates + a.deletes
}
func (a *familiesEngineAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fail := func(msg string) { a.errors = append(a.errors, msg); http.Error(w, msg, 400) }
	if r.Header.Get("Authorization") != engineToken {
		fail("unexpected token")
		return
	}
	if !strings.HasPrefix(r.URL.Path, a.base) {
		fail("unexpected route " + r.URL.Path)
		return
	}
	route := strings.TrimPrefix(r.URL.Path, a.base)
	if route == "read/missing-id" {
		if r.Method != "GET" {
			fail("non-GET read")
			return
		}
		http.NotFound(w, r)
		return
	}
	if route == "read/"+a.id {
		if r.Method != "GET" {
			fail("non-GET read")
			return
		}
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(a.remote)
		return
	}
	if r.Method != "POST" {
		fail("non-POST write")
		return
	}
	if route == "delete/"+a.id {
		a.deletes++
		if a.family == "dataprotection" {
			a.remote["scoped"] = false
			a.remote["masks"] = []any{}
		} else {
			a.exists = false
		}
		switch a.family {
		case "authorization":
			// The backend acknowledges authorization deletion with an empty body.
		case "group":
			json.NewEncoder(w).Encode(map[string]any{"status": "team deleted successfully"})
		case "script":
			json.NewEncoder(w).Encode("success")
		case "schedule":
			json.NewEncoder(w).Encode(map[string]any{"id": a.id, "message": "schedule deleted"})
		case "dataprotection":
			json.NewEncoder(w).Encode(map[string]any{"id": a.id, "message": "data protection turned off"})
		}
		return
	}
	create := route == "create"
	if !create && route != "update/"+a.id {
		fail("unexpected operation " + route)
		return
	}
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail("invalid request JSON")
		return
	}
	allowed := map[string][]string{
		"authorization":  {"name", "description", "resource", "resourceType", "permissions"},
		"group":          {"Name", "Members", "Endpoints", "SlackChannelID"},
		"script":         {"Name", "Command", "Endpoint", "Description", "ParameterDescriptions"},
		"schedule":       {"name", "description", "scheduleType", "isActive", "allDay", "startHour", "startMinute", "endHour", "endMinute", "weekdays", "startDay", "endDay", "specificDates", "users", "teams", "endpoints", "expiresAt", "maxAccessTime", "timezone", "operationType"},
		"dataprotection": {"resourceName", "scoped", "masks"},
	}
	for k := range req {
		found := false
		for _, valid := range allowed[a.family] {
			if k == valid {
				found = true
			}
		}
		if !found {
			fail("unexpected wire key " + k)
			return
		}
	}
	if create && a.exists {
		http.Error(w, "duplicate create", 409)
		return
	}
	if !create && !a.exists {
		http.NotFound(w, r)
		return
	}
	if !create && a.family == "group" && a.failSlack {
		http.Error(w, "Slack followup failed", 500)
		return
	}
	if create {
		a.creates++
		a.exists = true
	} else {
		a.updates++
	}
	a.remote["id"] = a.id
	switch a.family {
	case "authorization":
		a.remote["name"] = req["name"]
		a.remote["status"] = "created"
		a.remote["permissions"] = req["permissions"]
		a.remote["type"] = "custom"
		if create {
			a.remote["resourceType"] = req["resource"]
		} else {
			a.remote["resourceType"] = req["resourceType"]
		}
		if create || req["description"] != "" {
			a.remote["description"] = req["description"]
		}
	case "group":
		a.remote["name"] = req["Name"]
		a.remote["members"] = req["Members"]
		a.remote["endpoints"] = req["Endpoints"]
		if !create {
			a.remote["slackChannelId"] = req["SlackChannelID"]
		}
	case "script":
		a.remote["name"] = req["Name"]
		if create {
			a.remote["endpoint"] = req["Endpoint"]
		}
		a.remote["commandOmitted"] = true
		a.remote["commandDigest"] = "fingerprint:" + fmt.Sprint(req["Command"])
		if create || req["Description"] != "" {
			a.remote["description"] = req["Description"]
		}
		if v, ok := req["ParameterDescriptions"]; ok && v != nil {
			a.remote["parameterDescriptions"] = v
		}
	case "schedule":
		for _, k := range []string{"name", "scheduleType", "isActive", "allDay", "startHour", "startMinute", "endHour", "endMinute", "weekdays", "startDay", "endDay", "specificDates", "description", "expiresAt", "operationType"} {
			v := req[k]
			if v == nil {
				switch k {
				case "description", "expiresAt":
					v = ""
				case "allDay":
					v = false
				case "startDay", "endDay":
					v = 0
				case "weekdays", "specificDates":
					v = []any{}
				}
			}
			a.remote[k] = v
		}
		for _, k := range []string{"users", "teams", "endpoints"} {
			v := req[k]
			if v == nil {
				v = []any{}
			}
			a.remote[k] = v
		}
		if create {
			a.remote["timezone"] = req["timezone"]
		}
		if v, ok := req["maxAccessTime"]; ok {
			a.remote["maxAccessTime"] = v
		}
	case "dataprotection":
		a.remote["resourceName"] = req["resourceName"]
		a.remote["scoped"] = req["scoped"]
		a.remote["masks"] = req["masks"]
		a.remote["authorizationName"] = "masked_db"
		a.remote["status"] = "created"
		a.remote["lastAppliedAt"] = ""
	}
	if a.family == "schedule" || a.family == "dataprotection" {
		json.NewEncoder(w).Encode(a.remote)
	} else if create {
		json.NewEncoder(w).Encode(map[string]any{"id": a.id})
	} else if a.family == "authorization" {
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	} else if a.family == "group" {
		json.NewEncoder(w).Encode(map[string]any{"status": "team updated successfully"})
	} else {
		json.NewEncoder(w).Encode("success")
	}
}
func (a *familiesEngineAPI) assert(t *testing.T, stage int) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists {
		t.Fatal("expected remote object")
	}
	check := func(key string, want any) {
		if !reflect.DeepEqual(a.remote[key], want) {
			t.Fatalf("%s remote %s does not match configured lifecycle stage %d", a.family, key, stage)
		}
	}
	if a.family != "dataprotection" {
		check("name", "family-test")
	}
	if a.family == "script" {
		check("endpoint", "db")
	}
	if a.family != "group" && a.family != "dataprotection" {
		description := "initial"
		if stage > 0 {
			description = "updated"
		}
		if a.family == "schedule" && stage == 2 {
			description = ""
		}
		check("description", description)
	}
	switch a.family {
	case "authorization":
		want := "SELECT"
		if stage > 0 {
			want = "UPDATE"
		}
		check("permissions", want)
		check("resourceType", "mysql")
	case "group":
		members := []any{"member@example.invalid"}
		endpoints := []any{"db"}
		slack := "C123"
		if stage > 0 {
			members = []any{"other@example.invalid"}
		}
		if stage == 2 {
			members = []any{}
			endpoints = []any{}
			slack = ""
		}
		check("members", members)
		check("endpoints", endpoints)
		check("slackChannelId", slack)
	case "script":
		value := "initial"
		if stage > 0 {
			value = "updated"
		}
		check("commandDigest", "fingerprint:echo "+value)
		want := map[string]any{"path": value}
		if stage == 2 {
			want = map[string]any{}
		}
		check("parameterDescriptions", want)
	case "schedule":
		check("timezone", "UTC")
		if stage == 2 {
			check("users", []any{})
			check("teams", []any{})
			check("endpoints", []any{})
			check("description", "")
			check("expiresAt", "")
			check("startHour", float64(0))
			check("isActive", false)
			check("allDay", true)
			check("maxAccessTime", float64(30))
		} else {
			hour := float64(9)
			if stage > 0 {
				hour = 10
			}
			check("startHour", hour)
			check("maxAccessTime", float64(60))
		}
	case "dataprotection":
		check("scoped", stage == 0)
		if stage == 2 {
			check("masks", []any{})
		} else {
			check("masks", []any{map[string]any{"databaseName": "app", "tables": []any{map[string]any{"tableName": "people", "masked": true, "maskedColumns": []any{map[string]any{"columnName": "email", "maskingType": "email"}}}}}})
		}
	}
}
