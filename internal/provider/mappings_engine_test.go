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

	"gopkg.in/yaml.v2"
)

// TestMappingsTerraformEngine exercises persisted state and plans with a real
// Terraform CLI. Reuse the secret engine's isolated HOME/environment, local dev
// override and no-init policy; never contact a registry or a real Adaptive API.
// ADAPTIVE_ENGINE_PROVIDER_BINARY can select an independently built baseline.
func TestMappingsTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform on PATH")
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline provider build failed: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine provider binary must be named terraform-provider-adaptive")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Fatal("engine provider binary is missing")
	}

	for _, typ := range []string{"clickhouse", "mongodb36"} {
		t.Run(typ+"_import_omitted_tls_and_explicit_disable", func(t *testing.T) {
			h, api := newMappingsEngine(t, terraform, bin, typ, true)
			config := func(tag, tls string) { mappingsConfig(h, typ, tag, mappingsArguments(typ)+"\n"+tls) }
			config("initial", "")
			h.run(0, "import", "adaptive_resource.test", engineID)
			mappingsState(t, h, "use_tls", true)
			t.Run("import_has_clean_plan", func(t *testing.T) {
				imported := *h
				imported.t = t
				mappingsClean(&imported, api, 0)
			})
			config("changed", "")
			api.expect(api.configuration())
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 1, "changed")
			mappingsState(t, h, "use_tls", true)
			mappingsClean(h, api, 1)
			config("changed", "use_tls = false")
			expected := api.configuration()
			tlsKey := "use-tls"
			if typ == "mongodb36" {
				tlsKey = "useTLS"
			}
			expected[tlsKey] = false
			api.expect(expected)
			h.run(2, "plan", "-detailed-exitcode", "-out=disable.tfplan")
			h.run(0, "apply", "-auto-approve", "disable.tfplan")
			mappingsState(t, h, "use_tls", false)
			mappingsClean(h, api, 2)
			// MongoDB's bool,omitempty intentionally omits false. The fake backend
			// normalizes an absent useTLS to false, rather than requiring a wire key.
			api.mu.Lock()
			api.cfg[tlsKey] = true
			api.mu.Unlock()
			h.run(2, "plan", "-detailed-exitcode", "-out=drift.tfplan")
			mappingsPlanUpdate(t, h, "drift.tfplan", "use_tls", true, false)
			api.mu.Lock()
			unchanged := api.mutations == 2 && len(api.writes) == 2 && api.cfg[tlsKey] == true
			api.mu.Unlock()
			if !unchanged {
				t.Fatal("drift plan mutated backend or erased external TLS change")
			}
			h.run(0, "apply", "-auto-approve", "drift.tfplan")
			mappingsClean(h, api, 3)
			api.assert(t, 3, "changed")
		})
	}

	for _, alias := range []string{"host", "hostname"} {
		t.Run("aruba_"+alias+"_canonical_create_update", func(t *testing.T) {
			h, api := newMappingsEngine(t, terraform, bin, "aruba_instant_on", false)
			args := fmt.Sprintf("%s = %q\nusername = %q\nport = %q\nwebui_port = %q\nlogin_url = %q", alias, "switch.invalid", "engine-user", "22", "8443", "/login")
			mappingsConfig(h, "aruba_instant_on", "initial", args)
			api.expect(api.configuration())
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 1, "initial")
			mappingsState(t, h, alias, "switch.invalid")
			other := "hostname"
			if alias == "hostname" {
				other = "host"
			}
			mappingsState(t, h, other, "")
			mappingsClean(h, api, 1)
			mappingsConfig(h, "aruba_instant_on", "changed", args)
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 2, "changed")
			// Repeated refreshed plans catch perpetual two-alias churn.
			mappingsClean(h, api, 2)
			mappingsClean(h, api, 2)
		})
	}

	t.Run("redshift_ssl_mode_full_replacement_and_change", func(t *testing.T) {
		h, api := newMappingsEngine(t, terraform, bin, "awsredshift", true)
		config := func(tag, mode string) {
			mappingsConfig(h, "awsredshift", tag, mappingsArguments("awsredshift")+fmt.Sprintf("\nssl_mode = %q", mode))
		}
		config("initial", "verify-full")
		h.run(0, "import", "adaptive_resource.test", engineID)
		mappingsState(t, h, "ssl_mode", "verify-full")
		t.Run("import_has_clean_plan", func(t *testing.T) {
			imported := *h
			imported.t = t
			mappingsClean(&imported, api, 0)
		})
		config("changed", "verify-full")
		api.expect(api.configuration())
		h.run(0, "apply", "-auto-approve")
		api.assert(t, 1, "changed")
		mappingsClean(h, api, 1)
		config("changed", "require")
		expected := api.configuration()
		expected["sslMode"] = "require"
		api.expect(expected)
		h.run(2, "plan", "-detailed-exitcode", "-out=ssl.tfplan")
		mappingsPlanUpdate(t, h, "ssl.tfplan", "ssl_mode", "verify-full", "require")
		h.run(0, "apply", "-auto-approve", "ssl.tfplan")
		mappingsState(t, h, "ssl_mode", "require")
		mappingsClean(h, api, 2)
		api.assert(t, 2, "changed")
		// Backend non-proxy execution's unconditional TLS require is outside the
		// provider contract: this test verifies wire emission and convergence only.
	})

	for _, tc := range []struct{ name, typ, args, diagnostic string }{
		{"clickhouse_ssl_mode", "clickhouse", mappingsArguments("clickhouse") + "\nssl_mode = \"require\"", "use_tls"},
		{"aruba_api_token", "aruba_instant_on", "host = \"switch.invalid\"\napi_token = " + fmt.Sprintf("%q", enginePassword), "api_token"},
		{"aruba_host_alias_conflict", "aruba_instant_on", "host = \"switch.invalid\"\nhostname = \"different.invalid\"\npassword = " + fmt.Sprintf("%q", enginePassword), "host and hostname"},
	} {
		t.Run("reject_"+tc.name+"_during_plan", func(t *testing.T) {
			h, api := newMappingsEngine(t, terraform, bin, tc.typ, false)
			mappingsConfig(h, tc.typ, "initial", tc.args)
			out := h.run(1, "plan", "-detailed-exitcode")
			if !strings.Contains(out, tc.diagnostic) {
				t.Fatalf("wrong plan diagnostic:\n%s", engineSafe(out))
			}
			for _, secret := range []string{enginePassword, engineRotated, engineToken} {
				if strings.Contains(out, secret) {
					t.Fatal("plan diagnostic exposed a credential canary")
				}
			}
			api.assert(t, 0, "initial")
		})
	}
}

func mappingsArguments(typ string) string {
	if typ == "mongodb36" {
		return "uri = \"mongodb://database.invalid/engine-db\""
	}
	port := "9440"
	if typ == "awsredshift" {
		port = "5439"
	}
	return fmt.Sprintf("host = %q\nport = %q\nusername = %q\ndatabase_name = %q", "database.invalid", port, "engine-user", "engine-db")
}

func mappingsConfig(h *secretEngine, typ, tag, args string) {
	h.t.Helper()
	engineWrite(h.t, filepath.Join(h.dir, "main.tf"), fmt.Sprintf(`terraform {
 required_providers {
  adaptive = { source = "adaptive-scale/adaptive" }
 }
}
provider "adaptive" {
 service_token = %q
 workspace_url = %q
}
resource "adaptive_resource" "test" {
 type = %q
 name = "engine-resource"
 tags = [%q]
 %s
}
`, engineToken, h.url, typ, tag, args))
}

func mappingsState(t *testing.T, h *secretEngine, key string, want any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Resources []struct {
			Instances []struct {
				Attributes map[string]any `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(data, &state) != nil || len(state.Resources) != 1 || len(state.Resources[0].Instances) != 1 {
		t.Fatal("unexpected persisted Terraform state shape")
	}
	got := state.Resources[0].Instances[0].Attributes[key]
	// Unconfigured optional aliases may persist as null rather than "".
	if got == nil && want == "" {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted %s does not match expected mapping", key)
	}
}

func mappingsPlanUpdate(t *testing.T, h *secretEngine, file, key string, before, after any) {
	t.Helper()
	var plan struct {
		Changes []struct {
			Address string `json:"address"`
			Change  struct {
				Actions       []string `json:"actions"`
				Before, After map[string]any
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if json.Unmarshal([]byte(h.run(0, "show", "-json", file)), &plan) != nil {
		t.Fatal("cannot decode actual Terraform plan")
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Address != "adaptive_resource.test" || !reflect.DeepEqual(plan.Changes[0].Change.Actions, []string{"update"}) || !reflect.DeepEqual(plan.Changes[0].Change.Before[key], before) || !reflect.DeepEqual(plan.Changes[0].Change.After[key], after) {
		t.Fatalf("actual plan does not restore/change %s", key)
	}
}

func mappingsClean(h *secretEngine, api *mappingsEngineAPI, mutations int) {
	h.t.Helper()
	api.mu.Lock()
	reads := api.reads
	tag := api.tags[0]
	api.mu.Unlock()
	h.run(0, "plan", "-detailed-exitcode")
	api.assert(h.t, mutations, tag)
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.reads <= reads {
		h.t.Fatal("clean plan did not actually refresh from fake backend")
	}
}

type mappingsEngineAPI struct {
	mu               sync.Mutex
	typ              string
	exists           bool
	cfg, expected    map[string]any
	tags             []string
	mutations, reads int
	calls, errors    []string
	writes           []map[string]any
}

func newMappingsEngine(t *testing.T, terraform, bin, typ string, exists bool) (*secretEngine, *mappingsEngineAPI) {
	t.Helper()
	api := &mappingsEngineAPI{typ: typ, exists: exists, tags: []string{"initial"}}
	switch typ {
	case "clickhouse":
		api.cfg = map[string]any{"name": "engine-resource", "hostname": "database.invalid", "port": "9440", "username": "engine-user", "password": "", "database": "engine-db", "use-tls": true}
	case "mongodb36":
		api.cfg = map[string]any{"version": "1.0", "name": "engine-resource", "uri": "mongodb://database.invalid/engine-db", "useTLS": true}
	case "aruba_instant_on":
		api.cfg = map[string]any{"version": "1.0", "name": "engine-resource", "hostname": "switch.invalid", "username": "engine-user", "password": "", "sshKey": "", "usePassword": true, "port": "22", "webui_port": "8443", "login_url": "/login"}
	case "awsredshift":
		api.cfg = map[string]any{"version": "", "name": "engine-resource", "hostname": "database.invalid", "port": "5439", "username": "engine-user", "password": "", "databaseName": "engine-db", "sslMode": "verify-full"}
	default:
		t.Fatal("unsupported fake integration")
	}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	h := newSecretEngine(t, terraform, bin, false)
	h.url = server.URL
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) != 0 {
			t.Errorf("mapping fake API contract violations: %v", api.errors)
		}
		t.Logf("mapping API calls: %s; reads=%d mutations=%d recorded_writes=%d", strings.Join(api.calls, ", "), api.reads, api.mutations, len(api.writes))
	})
	return h, api
}

func mappingsCopy(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
func (a *mappingsEngineAPI) configuration() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return mappingsCopy(a.cfg)
}
func (a *mappingsEngineAPI) expect(cfg map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expected = mappingsCopy(cfg)
}
func (a *mappingsEngineAPI) assert(t *testing.T, mutations int, tag string) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.errors) != 0 || a.mutations != mutations || len(a.writes) != mutations || !reflect.DeepEqual(a.tags, []string{tag}) {
		t.Fatalf("fake backend mismatch: mutations=%d want=%d writes=%d tags_match=%t violations=%v", a.mutations, mutations, len(a.writes), reflect.DeepEqual(a.tags, []string{tag}), a.errors)
	}
	if mutations > 0 && !reflect.DeepEqual(a.cfg, a.expected) {
		t.Fatal("actual backend configuration did not converge to expected full replacement")
	}
}

func (a *mappingsEngineAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) {
		a.errors = append(a.errors, message)
		http.Error(w, message, http.StatusBadRequest)
	}
	if r.Header.Get("Authorization") != engineToken {
		fail("missing explicit fake authorization")
		return
	}
	if r.Method == "GET" && r.URL.Path == engineAPI+"read/"+engineID {
		a.reads++
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "name": "engine-resource", "integrationType": a.typ, "configuration": a.cfg, "userTags": a.tags, "redactedKeys": []string{}, "redactedDigests": map[string]string{}})
		return
	}
	create, update := r.URL.Path == engineAPI+"create", r.URL.Path == engineAPI+"update/"+engineID
	if r.Method != "POST" || (!create && !update) {
		fail("unexpected endpoint/method")
		return
	}
	if create == a.exists {
		fail("incorrect create/update lifecycle")
		return
	}
	var request struct {
		Name   string   `json:"name"`
		Type   string   `json:"integrationType"`
		Config string   `json:"config"`
		Tags   []string `json:"userTags"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil || request.Type != a.typ || (create && request.Name != "engine-resource") {
		fail("invalid mutation DTO")
		return
	}
	var cfg map[string]any
	if yaml.UnmarshalStrict([]byte(request.Config), &cfg) != nil {
		fail("invalid or duplicate YAML keys")
		return
	}
	// Missing MongoDB bool,omitempty is the backend's false, not an error.
	if a.typ == "mongodb36" {
		if _, ok := cfg["useTLS"]; !ok {
			cfg["useTLS"] = false
		}
	}
	if !reflect.DeepEqual(cfg, a.expected) {
		fail("full replacement YAML violates independent backend field/value contract")
		return
	}
	// Equality above enforces bool (not string) TLS, canonical keys and empty
	// authoritative credentials; obsolete aliases/extra fields are rejected.
	a.cfg, a.tags, a.exists = mappingsCopy(cfg), request.Tags, true
	a.writes = append(a.writes, mappingsCopy(cfg))
	a.mutations++
	if update {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	} else {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
	}
}
