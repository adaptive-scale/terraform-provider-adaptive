package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// TestCoreLifecycleTerraformEngine closes the Resource/Endpoint lifecycle gaps
// using actual Terraform subprocesses, persisted state and saved plans. The
// loopback fixtures use canonical backend 52bce48e wire contracts only (no
// legacy aliases or provider response structs). No init, registry or live API.
// Run: ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^TestCoreLifecycleTerraformEngine$' -v -count=1
// ADAPTIVE_ENGINE_PROVIDER_BINARY optionally selects an already-built plugin.
func TestCoreLifecycleTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	tf, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform on PATH")
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir, cmd.Env = "../..", append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline plugin build: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine binary must be named terraform-provider-adaptive")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Fatal("engine provider binary is missing")
	}
	for _, family := range []string{"Resource", "Endpoint"} {
		t.Run(family, func(t *testing.T) {
			h, api := newCoreLifecycleEngine(t, tf, bin, family == "Endpoint")
			address := "adaptive_resource.test"
			if api.endpoint != nil {
				address = endpointEngineAddress
			}
			api.config(h, "initial")
			h.run(0, "apply", "-auto-approve")
			api.assert(t, 1, 0, 0, 0, "initial")
			familiesEngineState(h, address, "id", engineID)
			api.config(h, "changed")
			api.plan(h, 2, "update.tfplan")
			coreLifecycleActions(h, address, "update.tfplan", "update")
			h.run(0, "apply", "-auto-approve", "update.tfplan")
			api.assert(t, 1, 1, 0, 0, "changed")
			for cycle := 0; cycle < 2; cycle++ {
				api.pure(t, func() { h.run(0, "apply", "-refresh-only", "-auto-approve") })
				familiesEngineState(h, address, "id", engineID)
				familiesEngineState(h, address, "tags", []any{"changed"})
				if api.endpoint != nil {
					policy := endpointEnginePolicy(false)
					policy["jitMode"] = "session"
					endpointEngineState(h, policy)
				}
			}
			api.plan(h, 0, "")
			api.pure(t, func() { familiesEngineForget(h, address) })
			coreLifecycleEmptyState(h)
			api.pure(t, func() { h.run(0, "import", address, engineID) })
			familiesEngineState(h, address, "id", engineID)
			api.plan(h, 0, "")
			api.pure(t, func() { familiesEngineForget(h, address) })
			api.pure(t, func() {
				out := h.run(1, "import", address, "missing-id")
				if !strings.Contains(out, "Cannot import non-existent remote object") {
					t.Fatalf("wrong missing-import diagnostic:\n%s", engineSafe(out))
				}
			})
			coreLifecycleEmptyState(h)
			api.pure(t, func() { h.run(0, "import", address, engineID) })
			api.assert(t, 1, 1, 0, 0, "changed")
			api.disappear() // External actor, not a Terraform delete request.
			api.plan(h, 2, "recreate.tfplan")
			coreLifecycleActions(h, address, "recreate.tfplan", "create")
			api.assertAbsent(t)
			h.run(0, "apply", "-auto-approve", "recreate.tfplan")
			api.assert(t, 2, 1, 0, 0, "changed")
			familiesEngineState(h, address, "id", engineID)
			api.plan(h, 0, "")
			h.run(0, "destroy", "-auto-approve")
			forces := 0
			if api.endpoint != nil {
				forces = 1
			}
			api.assert(t, 2, 1, 1, forces, "changed")
			// Verify the exact remote route independently after destroy, not just an
			// in-memory exists bit or Terraform's successful exit status.
			req, err := http.NewRequest("GET", h.url+api.base+"read/"+engineID, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", engineToken)
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("authoritative post-destroy read: HTTP %d", resp.StatusCode)
			}
			api.assertAbsent(t)
			coreLifecycleEmptyState(h)
			api.assert(t, 2, 1, 1, forces, "changed")
			t.Logf("verified %s: creates=2 updates=1 deletes=1 force_deletes=%d; missing import rejected; recreate saved plan applied; remote 404 and empty persisted state", family, forces)
		})
	}
}

func coreLifecycleActions(h *secretEngine, address, file, action string) {
	h.t.Helper()
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
	if len(plan.Changes) != 1 || plan.Changes[0].Address != address || !reflect.DeepEqual(plan.Changes[0].Change.Actions, []string{action}) {
		h.t.Fatalf("saved plan must contain exactly one %s for %s", action, address)
	}
}
func coreLifecycleEmptyState(h *secretEngine) {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
	if err != nil {
		h.t.Fatal(err)
	}
	var state struct {
		Resources []json.RawMessage `json:"resources"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		h.t.Fatal(err)
	}
	if len(state.Resources) != 0 {
		h.t.Fatal("persisted Terraform state still contains resources")
	}
}

type coreLifecycleAPI struct {
	mu                                       sync.Mutex
	base                                     string
	endpoint                                 *endpointEngineAPI
	exists                                   bool
	remote                                   map[string]any
	creates, updates, deletes, forces, reads int
	pendingRead, terminalRead                bool
	errors, calls                            []string
}

func newCoreLifecycleEngine(t *testing.T, tf, bin string, endpoint bool) (*secretEngine, *coreLifecycleAPI) {
	var h *secretEngine
	api := &coreLifecycleAPI{base: engineAPI}
	if endpoint {
		// Reuse the strict canonical policy validator/read model without editing it.
		h, api.endpoint = newEndpointEngine(t, tf, bin, false)
		api.base = endpointEngineAPIPath
	} else {
		h = newSecretEngine(t, tf, bin, false)
	}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	h.url = srv.URL
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) != 0 {
			t.Errorf("core lifecycle contract violations: %v", api.errors)
		}
		t.Logf("core lifecycle API calls: %s", strings.Join(api.calls, ", "))
	})
	return h, api
}
func (a *coreLifecycleAPI) config(h *secretEngine, tag string) {
	if a.endpoint != nil {
		endpointEngineConfig(h, "direct", tag, endpointEngineBoolConfig(false)+`
auto_approval = false
jit_multi_approver = false
jit_total_approvers = 0
max_jit_request_duration = ""
jit_mode = "session"`)
		return
	}
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
 name = "engine-resource"
 type = "clickhouse"
 host = "database.invalid"
 port = "9000"
 username = "engine-user"
 database_name = "engine-db"
 use_tls = false
 tags = [%q]
}
`, engineToken, h.url, tag))
}
func (a *coreLifecycleAPI) snapshot() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var ep any
	if a.endpoint != nil {
		a.endpoint.mu.Lock()
		defer a.endpoint.mu.Unlock()
		ep = []any{a.endpoint.exists, a.endpoint.remote, len(a.endpoint.writes)}
	}
	data, err := json.Marshal([]any{a.exists, a.remote, a.creates, a.updates, a.deletes, a.forces, ep})
	if err != nil {
		panic(err)
	}
	return string(data)
}
func (a *coreLifecycleAPI) pure(t *testing.T, operation func()) {
	t.Helper()
	before := a.snapshot()
	operation()
	if a.snapshot() != before {
		t.Fatal("read-only Terraform operation mutated remote state")
	}
}
func (a *coreLifecycleAPI) plan(h *secretEngine, code int, file string) {
	h.t.Helper()
	a.mu.Lock()
	reads := a.reads
	a.mu.Unlock()
	a.pure(h.t, func() {
		args := []string{"plan", "-detailed-exitcode"}
		if file != "" {
			args = append(args, "-out="+file)
		}
		h.run(code, args...)
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reads <= reads {
		h.t.Fatal("plan did not refresh authoritative remote state")
	}
}
func (a *coreLifecycleAPI) disappear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.exists = false
	if a.endpoint != nil {
		a.endpoint.mu.Lock()
		a.endpoint.exists = false
		a.endpoint.mu.Unlock()
	}
}
func (a *coreLifecycleAPI) assertAbsent(t *testing.T) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.exists {
		t.Fatal("remote object remains present")
	}
	if a.endpoint != nil {
		a.endpoint.mu.Lock()
		defer a.endpoint.mu.Unlock()
		if a.endpoint.exists {
			t.Fatal("endpoint row remains present")
		}
	}
}
func (a *coreLifecycleAPI) assert(t *testing.T, creates, updates, deletes, forces int, tag string) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.errors) != 0 || a.creates != creates || a.updates != updates || a.deletes != deletes || a.forces != forces || a.pendingRead {
		t.Fatalf("lifecycle creates/updates/deletes/forces=%d/%d/%d/%d want=%d/%d/%d/%d pending_read=%t errors=%v", a.creates, a.updates, a.deletes, a.forces, creates, updates, deletes, forces, a.pendingRead, a.errors)
	}
	if a.exists != (deletes == 0) {
		t.Fatal("unexpected remote existence after lifecycle operation")
	}
	if a.endpoint != nil {
		want := endpointEnginePolicy(false)
		want["jitMode"], want["userTags"] = "session", []any{tag}
		a.endpoint.assert(t, creates+updates, want)
		if forces > 0 && !a.terminalRead {
			t.Fatal("force deletion did not follow an authoritative terminal read")
		}
	} else if !reflect.DeepEqual(a.remote["userTags"], []any{tag}) {
		t.Fatal("resource tag update did not persist")
	}
}

func (a *coreLifecycleAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) {
		a.errors = append(a.errors, message)
		http.Error(w, message, http.StatusBadRequest)
	}
	if r.Header.Get("Authorization") != engineToken || r.URL.RawQuery != "" {
		fail("wrong authorization or unexpected query")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, a.base)
	if a.base+path != r.URL.Path {
		fail("noncanonical API path")
		return
	}
	if r.Method == "GET" && (path == "read/"+engineID || path == "read/missing-id") {
		a.reads++
		if path == "read/missing-id" || !a.exists {
			w.WriteHeader(http.StatusNotFound)
			message := "resource not found"
			if a.endpoint != nil {
				message = "session not found"
			}
			_ = json.NewEncoder(w).Encode(message)
			return
		}
		a.pendingRead = false
		if a.endpoint != nil {
			a.endpoint.mu.Lock()
			if a.endpoint.remote["status"] == "terminated" {
				a.terminalRead = true
			}
			a.endpoint.mu.Unlock()
			a.endpoint.ServeHTTP(w, r)
		} else {
			_ = json.NewEncoder(w).Encode(a.remote)
		}
		return
	}
	if r.Method != "POST" {
		fail("unexpected method")
		return
	}
	if path == "delete/"+engineID || path == "forcedelete/"+engineID {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 || !a.exists || a.pendingRead {
			fail("invalid/duplicate delete or unverified preceding mutation")
			return
		}
		if a.endpoint == nil {
			if path != "delete/"+engineID || a.deletes != 0 {
				fail("invalid resource delete")
				return
			}
			a.deletes++
			a.exists = false
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		} else if path == "delete/"+engineID {
			if a.deletes != 0 {
				fail("duplicate endpoint termination")
				return
			}
			a.deletes++
			a.endpoint.mutate(map[string]any{"status": "terminated"})
			// EndSessionSync returns a session, not an absence or hard-delete ACK.
			_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "status": "terminated"})
		} else {
			remote, _, _ := a.endpoint.snapshot()
			if a.deletes != 1 || a.forces != 0 || !a.terminalRead || remote["status"] != "terminated" {
				fail("force delete requires termination then terminal read")
				return
			}
			a.forces++
			a.exists = false
			a.endpoint.mu.Lock()
			a.endpoint.exists = false
			a.endpoint.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "session deleted successfully"})
		}
		return
	}
	create, update := path == "create", path == "update/"+engineID
	if (!create && !update) || create == a.exists || a.pendingRead {
		fail("invalid/duplicate mutation lifecycle")
		return
	}
	if a.endpoint != nil {
		a.endpoint.ServeHTTP(w, r)
		a.endpoint.mu.Lock()
		valid := len(a.endpoint.errors) == 0
		a.endpoint.mu.Unlock()
		if !valid {
			fail("endpoint canonical mutation rejected")
			return
		}
	} else {
		if r.Header.Get("Content-Type") != "application/json" {
			fail("mutation must be JSON")
			return
		}
		var req struct {
			Name    string   `json:"name"`
			Type    string   `json:"integrationType"`
			Config  string   `json:"config"`
			Tags    []string `json:"userTags"`
			Cluster string   `json:"defaultCluster"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&req) != nil || dec.Decode(new(any)) != io.EOF || req.Type != "clickhouse" || req.Cluster != "" || (create && req.Name != "engine-resource") || (!create && req.Name != "") || len(req.Tags) != 1 || (req.Tags[0] != "initial" && req.Tags[0] != "changed") {
			fail("invalid canonical resource write DTO")
			return
		}
		var cfg map[string]any
		want := map[string]any{"name": "engine-resource", "hostname": "database.invalid", "port": "9000", "username": "engine-user", "password": "", "database": "engine-db", "use-tls": false}
		if yaml.UnmarshalStrict([]byte(req.Config), &cfg) != nil || !reflect.DeepEqual(cfg, want) {
			fail("invalid full-replacement ClickHouse YAML")
			return
		}
		a.remote = map[string]any{"id": engineID, "name": "engine-resource", "integrationType": "clickhouse", "configuration": cfg, "userTags": []any{req.Tags[0]}, "redactedKeys": []any{}, "redactedDigests": map[string]any{}}
		if create {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		}
	}
	a.exists, a.pendingRead = true, true
	if create {
		a.creates++
	} else {
		a.updates++
	}
}
