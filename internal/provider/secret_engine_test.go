package provider_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

const (
	enginePassword = "engine-only-configured-password-canary"
	engineRotated  = "engine-only-external-password-canary"
	engineToken    = "engine-only-fake-service-token"
	engineID       = "engine-resource-1"
	engineAPI      = "/api/v1/terraform/resource/"
)

// TestSecretTerraformEngine uses a real locally built plugin and Terraform CLI,
// not SDK ResourceData simulations. No init, registry, live API or credentials.
// Run: ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^TestSecretTerraformEngine$' -v -count=1
// ADAPTIVE_ENGINE_PROVIDER_BINARY optionally selects a scratch-built baseline.
func TestSecretTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform on PATH")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("local provider build failed: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine provider binary must be named terraform-provider-adaptive")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Fatal("engine provider binary is missing")
	}

	t.Run("managed_rotation_survives_persisted_refreshes", func(t *testing.T) {
		h := newSecretEngine(t, terraform, bin, false)
		password := enginePassword
		h.config(&password, "initial")
		h.run(0, "apply", "-auto-approve")
		h.api.assert(t, enginePassword, []string{"initial"}, 1)
		baseline := h.state()
		if baseline.Password != enginePassword || baseline.Digests["password"] != engineDigest(enginePassword) {
			t.Fatal("initial apply did not retain configured legacy password and server digest")
		}
		h.api.mu.Lock()
		h.api.password = engineRotated
		h.api.mu.Unlock()
		for cycle := 1; cycle <= 2; cycle++ {
			before := h.api.mutationCount()
			h.run(0, "apply", "-refresh-only", "-auto-approve")
			state := h.state()
			if !reflect.DeepEqual(state.Digests, baseline.Digests) {
				t.Fatalf("refresh-only cycle %d acknowledged external rotation as provider write", cycle)
			}
			if state.Password != engineDigest(engineRotated) {
				t.Fatalf("refresh-only cycle %d lost the sensitive drift marker", cycle)
			}
			h.api.assert(t, engineRotated, []string{"initial"}, before)
		}
		h.run(2, "plan", "-detailed-exitcode", "-out=rotation.tfplan")
		var plan struct {
			Changes []struct {
				Address string `json:"address"`
				Change  struct {
					Actions []string       `json:"actions"`
					Before  map[string]any `json:"before"`
					After   map[string]any `json:"after"`
				} `json:"change"`
			} `json:"resource_changes"`
		}
		if err := json.Unmarshal([]byte(h.run(0, "show", "-json", "rotation.tfplan")), &plan); err != nil {
			t.Fatal("cannot decode actual Terraform plan JSON")
		}
		if len(plan.Changes) != 1 || plan.Changes[0].Address != "adaptive_resource.test" || !reflect.DeepEqual(plan.Changes[0].Change.Actions, []string{"update"}) || plan.Changes[0].Change.Before["password"] != engineDigest(engineRotated) || plan.Changes[0].Change.After["password"] != enginePassword {
			t.Fatal("actual Terraform plan did not propose restoring configured password")
		}
		h.run(0, "apply", "-auto-approve", "rotation.tfplan")
		h.api.assert(t, enginePassword, []string{"initial"}, 2)
		if h.state().Digests["password"] != engineDigest(enginePassword) {
			t.Fatal("successful apply did not capture the new server digest")
		}
		h.run(0, "plan", "-detailed-exitcode")
		h.api.assert(t, enginePassword, []string{"initial"}, 2)
	})

	t.Run("ignore_changes_rotation_blocks_tag_update", func(t *testing.T) {
		h := newSecretEngine(t, terraform, bin, false)
		password := enginePassword
		h.config(&password, "initial", "password")
		h.run(0, "apply", "-auto-approve")
		h.api.assert(t, enginePassword, []string{"initial"}, 1)
		h.api.mu.Lock()
		h.api.password = engineRotated
		h.api.mu.Unlock()
		h.config(&password, "changed", "password")
		out := h.run(1, "apply", "-auto-approve")
		if !strings.Contains(out, "ignore_changes") || !strings.Contains(out, "password") {
			t.Fatalf("ignored credential failed for wrong reason:\n%s", engineSafe(out))
		}
		h.api.assert(t, engineRotated, []string{"initial"}, 1)
	})

	t.Run("import_omission_guard_and_explicit_clear", func(t *testing.T) {
		h := newSecretEngine(t, terraform, bin, true)
		h.config(nil, "initial")
		h.run(0, "import", "adaptive_resource.test", engineID)
		if h.state().Digests["password"] != engineDigest(enginePassword) {
			t.Fatal("import did not seed server-held credential evidence")
		}
		h.config(nil, "changed")
		out := h.run(1, "apply", "-auto-approve")
		if !strings.Contains(out, "server-held values for password") {
			t.Fatalf("omitted imported password failed for wrong reason:\n%s", engineSafe(out))
		}
		h.api.assert(t, enginePassword, []string{"initial"}, 0)
		t.Log("omitted imported password rejected with zero server mutations")
		empty := ""
		h.config(&empty, "changed")
		h.run(0, "apply", "-auto-approve")
		h.api.assert(t, "", []string{"changed"}, 1)
		if len(h.state().Digests) != 0 {
			t.Fatal("explicit clear left stale applied digests")
		}
		h.config(nil, "after-clear")
		h.run(0, "apply", "-auto-approve")
		h.api.assert(t, "", []string{"after-clear"}, 2)
		h.run(0, "plan", "-detailed-exitcode")
		h.api.assert(t, "", []string{"after-clear"}, 2)
	})
}

type secretEngine struct {
	t                   *testing.T
	terraform, dir, url string
	env                 []string
	api                 *secretEngineAPI
}

func newSecretEngine(t *testing.T, terraform, bin string, existing bool) *secretEngine {
	t.Helper()
	api := &secretEngineAPI{exists: existing, password: enginePassword, tags: []string{"initial"}}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cli := filepath.Join(dir, "terraform.rc")
	engineWrite(t, cli, fmt.Sprintf("provider_installation {\n dev_overrides {\n  \"adaptive-scale/adaptive\" = %q\n }\n}\ndisable_checkpoint = true\n", filepath.Dir(bin)))
	// Deliberately do not inherit TF_*, ADAPTIVE_*, proxy, logging, token or
	// reattach variables. An empty HOME prevents all credential-file fallback.
	fakeHome := filepath.Join(dir, "home")
	if err := os.Mkdir(fakeHome, 0700); err != nil {
		t.Fatal(err)
	}
	h := &secretEngine{t: t, terraform: terraform, dir: dir, url: server.URL, api: api,
		env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + fakeHome, "TMPDIR=" + os.Getenv("TMPDIR"), "TF_CLI_CONFIG_FILE=" + cli, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1"}}
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) != 0 {
			t.Errorf("fake API contract violations: %v", api.errors)
		}
		t.Logf("verified API calls: %s; actual mutations=%d", strings.Join(api.calls, ", "), api.mutations)
	})
	return h
}

func (h *secretEngine) config(password *string, tag string, ignored ...string) {
	h.t.Helper()
	secret := ""
	if password != nil {
		secret = fmt.Sprintf("password = %q", *password)
	}
	if len(ignored) > 0 {
		secret += "\n lifecycle { ignore_changes = [" + strings.Join(ignored, ", ") + "] }"
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
  type = "mysql"
  name = "engine-resource"
  host = "database.invalid"
  port = "3306"
  username = "engine-user"
  database_name = "engine-db"
  tags = [%q]
  %s
}
`, engineToken, h.url, tag, secret))
}

func (h *secretEngine) run(want int, args ...string) string {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.terraform, append([]string{args[0], "-no-color"}, args[1:]...)...)
	cmd.Dir, cmd.Env = h.dir, h.env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			h.t.Fatalf("terraform launch failed: %v", err)
		}
	}
	if code != want {
		h.t.Fatalf("terraform %s: exit %d, want %d\n%s", strings.Join(args, " "), code, want, engineSafe(string(out)))
	}
	h.t.Logf("terraform %s: exit %d", strings.Join(args, " "), code)
	return string(out)
}

type engineSecretState struct {
	Password string
	Digests  map[string]string
}

func (h *secretEngine) state() engineSecretState {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
	if err != nil {
		h.t.Fatal(err)
	}
	var state struct {
		Resources []struct {
			Instances []struct {
				Attributes struct {
					Password string            `json:"password"`
					Digests  map[string]string `json:"applied_digests"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(data, &state) != nil || len(state.Resources) != 1 || len(state.Resources[0].Instances) != 1 {
		h.t.Fatal("unexpected persisted Terraform state shape")
	}
	a := state.Resources[0].Instances[0].Attributes
	return engineSecretState{a.Password, a.Digests}
}

// One resource, full configuration replacement, server-side redaction and keyed
// digests. Explicit clears are returned as password:"" without a digest.
type secretEngineAPI struct {
	mu            sync.Mutex
	exists        bool
	password      string
	tags          []string
	mutations     int
	calls, errors []string
}

func (a *secretEngineAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) { a.errors = append(a.errors, message); http.Error(w, message, 400) }
	if r.Header.Get("Authorization") != engineToken {
		fail("request did not use explicit fake token")
		return
	}
	if r.Method == "GET" && r.URL.Path == engineAPI+"read/"+engineID {
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		cfg := map[string]any{"name": "engine-resource", "hostname": "database.invalid", "port": "3306", "username": "engine-user", "databaseName": "engine-db", "sslMode": "require"}
		keys, digests := []string{}, map[string]string{}
		if a.password == "" {
			cfg["password"] = ""
		} else {
			keys = append(keys, "password")
			digests["password"] = engineDigest(a.password)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "name": "engine-resource", "integrationType": "mysql", "configuration": cfg, "userTags": a.tags, "redactedKeys": keys, "redactedDigests": digests})
		return
	}
	create := r.URL.Path == engineAPI+"create"
	update := r.URL.Path == engineAPI+"update/"+engineID
	if r.Method != "POST" || (!create && !update) {
		fail("unexpected method or endpoint")
		return
	}
	if create == a.exists {
		fail("invalid create/update lifecycle")
		return
	}
	var request struct {
		Name   string   `json:"name"`
		Type   string   `json:"integrationType"`
		Config string   `json:"config"`
		Tags   []string `json:"userTags"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil || request.Type != "mysql" || (create && request.Name != "engine-resource") {
		fail("invalid write DTO")
		return
	}
	var cfg map[string]any
	if yaml.Unmarshal([]byte(request.Config), &cfg) != nil {
		fail("invalid YAML configuration")
		return
	}
	password, ok := cfg["password"].(string)
	if !ok {
		fail("full-replacement write missing password string")
		return
	}
	if password != "" && password != enginePassword {
		fail("write attempted unexpected credential or digest marker")
		return
	}
	a.exists, a.password, a.tags = true, password, request.Tags
	a.mutations++
	if update {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	} else {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
	}
}
func (a *secretEngineAPI) mutationCount() int { a.mu.Lock(); defer a.mu.Unlock(); return a.mutations }
func (a *secretEngineAPI) assert(t *testing.T, password string, tags []string, mutations int) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.password != password || !reflect.DeepEqual(a.tags, tags) || a.mutations != mutations {
		t.Fatalf("actual server state mismatch: password_matches=%t tags_match=%t mutations=%d want=%d", a.password == password, reflect.DeepEqual(a.tags, tags), a.mutations, mutations)
	}
}
func engineDigest(password string) string {
	h := hmac.New(sha256.New, []byte("engine-only-synthetic-HMAC-key"))
	_, _ = h.Write([]byte(engineID + ":" + password))
	return hex.EncodeToString(h.Sum(nil))
}
func engineSafe(out string) string {
	for _, value := range []string{enginePassword, engineRotated, engineToken, engineDigest(enginePassword), engineDigest(engineRotated)} {
		out = strings.ReplaceAll(out, value, "[REDACTED-TEST-CANARY]")
	}
	return out
}
func engineWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
