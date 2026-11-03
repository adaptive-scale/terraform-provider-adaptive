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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

// TestCloudTerraformEngine verifies configuration API behavior, not runtime AWS
// IAM permissions, session creation or connections. It never uses init, registry,
// live APIs or credentials. ADAPTIVE_ENGINE_PROVIDER_BINARY selects a baseline.
func TestCloudTerraformEngine(t *testing.T) {
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
			t.Fatalf("offline provider build: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine binary must be named terraform-provider-adaptive")
	}
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		t.Fatal("engine binary missing")
	}

	for _, typ := range []string{"aws", "keyspaces"} {
		for _, mode := range []string{"role", "service_account", "static", "both"} {
			t.Run(typ+"_"+mode+"_create_refresh_update", func(t *testing.T) {
				h := newCloudEngine(t, terraform, bin, typ, false, mode)
				h.config("initial", h.args(true), "")
				h.run(0, "apply", "-auto-approve")
				h.assert(1, "initial")
				h.clean(1)
				h.run(0, "apply", "-refresh-only", "-auto-approve")
				h.assert(1, "initial")
				h.config("changed", h.args(true), "")
				h.run(0, "apply", "-auto-approve")
				h.assert(2, "changed")
				h.clean(2)
				if typ == "keyspaces" {
					cfg := h.api.snapshot()
					cfg["keyspace"] = "updated"
					cfg["keyspaces_endpoint"] = "cassandra.us-west-2.amazonaws.com"
					cfg["version"] = "3.0"
					cfg["aws_region_name"] = "us-west-2"
					cfg["create_if_not_exists"] = false
					h.api.expect(cfg)
					h.config("changed", h.args(true), "")
					h.run(2, "plan", "-detailed-exitcode", "-out=fields.tfplan")
					h.run(0, "apply", "-auto-approve", "fields.tfplan")
					h.assert(3, "changed")
					h.clean(3)
				}
			})
		}
		t.Run(typ+"_import_matched_auth_and_opaque_credentials", func(t *testing.T) {
			h := newCloudEngine(t, terraform, bin, typ, true, "both")
			h.config("initial", h.args(false), "")
			h.run(0, "import", "adaptive_resource.test", engineID)
			h.clean(0)
			state := h.state()
			digests, ok := state["applied_digests"].(map[string]any)
			if !ok || digests["aws_access_key_id"] != cloudDigest(typ, "aws_access_key_id", enginePassword) || digests["aws_secret_access_key"] != cloudDigest(typ, "aws_secret_access_key", enginePassword) {
				t.Fatal("import did not retain held static credential evidence")
			}
			h.config("changed", h.args(false), "")
			out := h.run(1, "apply", "-auto-approve")
			h.diagnostic(out, "server-held", "key")
			h.assert(0, "initial")
		})
		t.Run(typ+"_static_rotation_ignored_marker_never_written", func(t *testing.T) {
			h := newCloudEngine(t, terraform, bin, typ, false, "static")
			h.config("initial", h.args(true), "")
			h.run(0, "apply", "-auto-approve")
			h.assert(1, "initial")
			baseline := h.state()["applied_digests"]
			cfg := h.api.snapshot()
			cfg["aws_access_key_id"], cfg["aws_secret_access_key"] = engineRotated, engineRotated
			h.api.seed(cfg)
			for i := 0; i < 2; i++ {
				h.run(0, "apply", "-refresh-only", "-auto-approve")
				state := h.state()
				id, secret := "access_key_id", "secret_access_key"
				if typ == "keyspaces" {
					id, secret = "aws_access_key_id", "aws_secret_access_key"
				}
				if !reflect.DeepEqual(state["applied_digests"], baseline) || state[id] != cloudDigest(typ, "aws_access_key_id", engineRotated) || state[secret] != cloudDigest(typ, "aws_secret_access_key", engineRotated) {
					t.Fatal("static credential refresh lost baseline or marker")
				}
				h.api.check(t, cfg, 1, "initial")
			}
			ignored := "access_key_id, secret_access_key"
			if typ == "keyspaces" {
				ignored = "aws_access_key_id, aws_secret_access_key"
			}
			h.config("changed", h.args(true), ignored)
			out := h.run(1, "apply", "-auto-approve")
			if !strings.Contains(out, "ignore_changes") || !strings.Contains(out, "key") {
				t.Fatalf("wrong ignored marker diagnostic:\n%s", engineSafe(out))
			}
			h.api.check(t, cfg, 1, "initial")
			h.api.mu.Lock()
			posts := h.api.posts
			h.api.mu.Unlock()
			if posts != 1 {
				t.Fatal("ignored marker update reached POST")
			}
		})
		t.Run(typ+"_import_tags_only_cannot_change_auth_or_policy", func(t *testing.T) {
			h := newCloudEngine(t, terraform, bin, typ, true, "both")
			// No held static credentials: this isolates unowned shared mode identifiers
			// and allowlist preservation rather than letting the secret guard mask loss.
			cfg := h.api.snapshot()
			cfg["aws_access_key_id"] = ""
			cfg["aws_secret_access_key"] = ""
			h.api.expect(cfg)
			h.api.seed(cfg)
			h.config("initial", "", "")
			h.run(0, "import", "adaptive_resource.test", engineID)
			h.config("changed", "", "")
			out, code := h.command("apply", "-auto-approve")
			if code == 0 {
				h.assert(1, "changed")
				h.clean(1)
			} else {
				if code != 1 {
					t.Fatal("unexpected Terraform exit code")
				}
				h.diagnostic(out, "config", "explicit")
				h.assert(0, "initial")
			}
		})
		t.Run(typ+"_old_absent_mode_flags_equal_false", func(t *testing.T) {
			h := newCloudEngine(t, terraform, bin, typ, true, "static")
			cfg := h.api.snapshot()
			cfg["aws_access_key_id"], cfg["aws_secret_access_key"] = "", ""
			h.api.seed(cfg)
			h.api.expect(cfg)
			h.api.mu.Lock()
			h.api.absentFalse = true
			h.api.mu.Unlock()
			h.config("initial", h.args(true), "")
			h.run(0, "import", "adaptive_resource.test", engineID)
			h.clean(0)
			h.config("changed", h.args(true), "")
			h.run(0, "apply", "-auto-approve")
			h.assert(1, "changed")
			h.clean(1)
		})
	}
	t.Run("aws_allowlist_explicit_clear_and_ignore_changes", func(t *testing.T) {
		h := newCloudEngine(t, terraform, bin, "aws", false, "role")
		h.config("initial", h.args(true), "")
		h.run(0, "apply", "-auto-approve")
		h.assert(1, "initial")
		// Ignoring a configured allowlist must retain the remote restriction on an
		// unrelated update, not accidentally emit a new or empty command list.
		cfg := h.api.snapshot()
		cfg["allowed_commands"] = "ec2:describe-instances"
		h.api.seed(cfg)
		h.api.expect(cfg)
		h.config("changed", h.argsWithCommands(true, "s3:ls"), "allowed_commands")
		h.run(0, "apply", "-auto-approve")
		h.assert(2, "changed")
		h.clean(2)
		cfg["enable_allowed_commands"], cfg["allowed_commands"] = false, ""
		h.api.expect(cfg)
		h.config("changed", h.args(true), "")
		h.run(2, "plan", "-detailed-exitcode", "-out=clear.tfplan")
		h.planUpdate("clear.tfplan", "enable_allowed_commands", true, false)
		h.planUpdate("clear.tfplan", "allowed_commands", "ec2:describe-instances", "")
		h.run(0, "apply", "-auto-approve", "clear.tfplan")
		h.assert(3, "changed")
		h.clean(3)
	})
	t.Run("datadog_application_rotation_saved_plan", func(t *testing.T) {
		h := newCloudEngine(t, terraform, bin, "datadog", false, "")
		h.config("initial", h.args(true), "")
		h.run(0, "apply", "-auto-approve")
		h.assert(1, "initial")
		baseline := h.state()
		cfg := h.api.snapshot()
		cfg["dd_app_key"] = engineRotated
		h.api.seed(cfg)
		for i := 0; i < 2; i++ {
			h.run(0, "apply", "-refresh-only", "-auto-approve")
			state := h.state()
			if !reflect.DeepEqual(state["applied_digests"], baseline["applied_digests"]) || state["app_key"] != cloudDigest("datadog", "dd_app_key", engineRotated) || state["dd_api_key"] != enginePassword {
				t.Fatal("refresh lost independent secret baseline or drift marker")
			}
			h.api.check(t, cfg, 1, "initial")
		}
		h.run(2, "plan", "-detailed-exitcode", "-out=rotation.tfplan")
		h.planUpdate("rotation.tfplan", "app_key", cloudDigest("datadog", "dd_app_key", engineRotated), enginePassword)
		h.planUpdate("rotation.tfplan", "dd_api_key", enginePassword, enginePassword)
		h.run(0, "apply", "-auto-approve", "rotation.tfplan")
		h.assert(2, "initial")
		h.clean(2)
	})
	t.Run("datadog_import_app_omission_guard_clear_then_omit", func(t *testing.T) {
		h := newCloudEngine(t, terraform, bin, "datadog", true, "")
		h.config("initial", h.args(false), "")
		h.run(0, "import", "adaptive_resource.test", engineID)
		h.clean(0)
		h.config("changed", fmt.Sprintf("dd_site = \"datadoghq.com\"\ndd_api_key = %q", enginePassword), "")
		out := h.run(1, "apply", "-auto-approve")
		h.diagnostic(out, "server-held", "app_key")
		h.assert(0, "initial")
		cfg := h.api.snapshot()
		cfg["dd_app_key"] = ""
		h.api.expect(cfg)
		h.config("changed", fmt.Sprintf("dd_site = \"datadoghq.com\"\ndd_api_key = %q\napp_key = \"\"", enginePassword), "")
		h.run(0, "apply", "-auto-approve")
		h.assert(1, "changed")
		h.clean(1)
		h.config("after-clear", fmt.Sprintf("dd_site = \"datadoghq.com\"\ndd_api_key = %q", enginePassword), "")
		h.run(0, "apply", "-auto-approve")
		h.assert(2, "after-clear")
		h.clean(2)
	})
}

type cloudEngine struct {
	*secretEngine
	api *cloudEngineAPI
	typ string
}

func newCloudEngine(t *testing.T, terraform, bin, typ string, existing bool, mode string) *cloudEngine {
	t.Helper()
	cfg := cloudFixture(typ, mode)
	api := &cloudEngineAPI{typ: typ, exists: existing, cfg: cloudCopy(cfg), expected: cloudCopy(cfg), tags: []string{"initial"}}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(dir, "terraform.rc")
	engineWrite(t, cli, fmt.Sprintf("provider_installation {\n dev_overrides {\n \"adaptive-scale/adaptive\" = %q\n }\n}\ndisable_checkpoint = true\n", filepath.Dir(bin)))
	h := &cloudEngine{secretEngine: &secretEngine{t: t, terraform: terraform, dir: dir, url: server.URL, env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + os.Getenv("TMPDIR"), "TF_CLI_CONFIG_FILE=" + cli, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1"}}, api: api, typ: typ}
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) > 0 {
			t.Errorf("fake backend contract violations: %s", strings.Join(api.errors, "; "))
		}
		t.Logf("configuration API reads=%d POST attempts=%d accepted mutations=%d (runtime IAM/sessions/connections not verified)", api.reads, api.posts, api.mutations)
	})
	return h
}
func cloudFixture(typ, mode string) map[string]any {
	if typ == "datadog" {
		return map[string]any{"dd_api_key": enginePassword, "dd_app_key": enginePassword, "dd_site": "datadoghq.com"}
	}
	c := map[string]any{"aws_region_name": "us-east-2", "aws_access_key_id": enginePassword, "aws_secret_access_key": enginePassword, "use_role_arn": mode == "role" || mode == "both", "aws_role_arn": "arn:aws:iam::123456789012:role/engine", "use_service_account": mode == "service_account" || mode == "both", "service_account": "engine-account", "create_if_not_exists": true, "version": "1.0"}
	if typ == "aws" {
		c["enable_allowed_commands"], c["allowed_commands"] = true, "s3:ls"
	} else {
		c["keyspace"], c["keyspaces_endpoint"], c["version"] = "application", "cassandra.us-east-2.amazonaws.com", "2.0"
	}
	return c
}
func (h *cloudEngine) args(credentials bool) string {
	h.api.mu.Lock()
	cfg := cloudCopy(h.api.expected)
	h.api.mu.Unlock()
	var lines []string
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if h.typ == "aws" && key == "version" {
			continue
		}
		if !credentials && cloudSecret(h.typ, key) {
			continue
		}
		attr := key
		if h.typ == "aws" {
			switch key {
			case "aws_region_name":
				attr = "region_name"
			case "aws_access_key_id":
				attr = "access_key_id"
			case "aws_secret_access_key":
				attr = "secret_access_key"
			}
		}
		if h.typ == "datadog" {
			if key == "dd_api_key" {
				attr = "dd_api_key"
			} else if key == "dd_app_key" {
				attr = "app_key"
			}
		}
		switch v := cfg[key].(type) {
		case string:
			lines = append(lines, fmt.Sprintf("%s = %q", attr, v))
		case bool:
			lines = append(lines, fmt.Sprintf("%s = %t", attr, v))
		}
	}
	return strings.Join(lines, "\n")
}
func (h *cloudEngine) argsWithCommands(credentials bool, commands string) string {
	args := h.args(credentials)
	h.api.mu.Lock()
	old := h.api.expected["allowed_commands"].(string)
	h.api.mu.Unlock()
	return strings.Replace(args, fmt.Sprintf("allowed_commands = %q", old), fmt.Sprintf("allowed_commands = %q", commands), 1)
}
func (h *cloudEngine) config(tag, args, ignored string) {
	h.t.Helper()
	lifecycle := ""
	if ignored != "" {
		lifecycle = "lifecycle { ignore_changes = [" + ignored + "] }"
	}
	engineWrite(h.t, filepath.Join(h.dir, "main.tf"), fmt.Sprintf(`terraform {
 required_providers { adaptive = { source = "adaptive-scale/adaptive" } }
}
provider "adaptive" {
 service_token = %q
 workspace_url = %q
}
resource "adaptive_resource" "test" {
 name = "engine-resource"
 type = %q
 tags = [%q]
 %s
 %s
}
`, engineToken, h.url, h.typ, tag, args, lifecycle))
}
func (h *cloudEngine) command(args ...string) (string, int) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.terraform, append([]string{args[0], "-no-color"}, args[1:]...)...)
	cmd.Dir, cmd.Env = h.dir, h.env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			h.t.Fatal("Terraform launch failed")
		}
	}
	h.t.Logf("terraform %s: exit %d", strings.Join(args, " "), code)
	return string(out), code
}
func (h *cloudEngine) run(want int, args ...string) string {
	h.t.Helper()
	out, code := h.command(args...)
	if code != want {
		h.t.Fatalf("Terraform %s exit=%d want=%d\n%s", strings.Join(args, " "), code, want, engineSafe(out))
	}
	return out
}
func (h *cloudEngine) clean(mutations int) {
	h.t.Helper()
	h.run(0, "plan", "-detailed-exitcode")
	h.api.mu.Lock()
	n := h.api.mutations
	h.api.mu.Unlock()
	if n != mutations {
		h.t.Fatal("plan mutated configuration API")
	}
}
func (h *cloudEngine) assert(n int, tag string) {
	h.t.Helper()
	h.api.mu.Lock()
	cfg := cloudCopy(h.api.expected)
	h.api.mu.Unlock()
	h.api.check(h.t, cfg, n, tag)
}
func (h *cloudEngine) diagnostic(out string, words ...string) {
	h.t.Helper()
	for _, s := range words {
		if !strings.Contains(strings.ToLower(out), s) {
			h.t.Fatalf("missing actionable diagnostic %q:\n%s", s, engineSafe(out))
		}
	}
	for _, s := range []string{enginePassword, engineRotated, engineToken} {
		if strings.Contains(out, s) {
			h.t.Fatal("diagnostic leaked synthetic credential")
		}
	}
	h.api.mu.Lock()
	posts := h.api.posts
	h.api.mu.Unlock()
	if posts != 0 {
		h.t.Fatal("unsafe update reached POST")
	}
}
func (h *cloudEngine) state() map[string]any {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
	if err != nil {
		h.t.Fatal(err)
	}
	var state struct {
		Resources []struct {
			Instances []struct {
				Attributes map[string]any `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(data, &state) != nil || len(state.Resources) != 1 || len(state.Resources[0].Instances) != 1 {
		h.t.Fatal("invalid persisted Terraform state")
	}
	return state.Resources[0].Instances[0].Attributes
}
func (h *cloudEngine) planUpdate(file, key string, before, after any) {
	h.t.Helper()
	var p struct {
		Changes []struct {
			Address string `json:"address"`
			Change  struct {
				Actions       []string `json:"actions"`
				Before, After map[string]any
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if json.Unmarshal([]byte(h.run(0, "show", "-json", file)), &p) != nil || len(p.Changes) != 1 {
		h.t.Fatal("invalid saved plan")
	}
	c := p.Changes[0]
	if c.Address != "adaptive_resource.test" || !reflect.DeepEqual(c.Change.Actions, []string{"update"}) || !reflect.DeepEqual(c.Change.Before[key], before) || !reflect.DeepEqual(c.Change.After[key], after) {
		h.t.Fatalf("saved plan did not contain intended %s update (values withheld)", key)
	}
}

// Independent backend key/type contract. Full replacement, never merge omitted
// values with prior config; response redaction is separate from request DTOs.
type cloudEngineAPI struct {
	mu                      sync.Mutex
	typ                     string
	exists, absentFalse     bool
	cfg, expected           map[string]any
	tags                    []string
	reads, posts, mutations int
	errors                  []string
}

func cloudCopy(c map[string]any) map[string]any {
	r := map[string]any{}
	for k, v := range c {
		r[k] = v
	}
	return r
}
func (a *cloudEngineAPI) snapshot() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloudCopy(a.cfg)
}
func (a *cloudEngineAPI) expect(c map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expected = cloudCopy(c)
}
func (a *cloudEngineAPI) seed(c map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cloudCopy(c)
}
func cloudSecret(typ, key string) bool {
	if typ == "datadog" {
		return key == "dd_api_key" || key == "dd_app_key"
	}
	return key == "aws_access_key_id" || key == "aws_secret_access_key"
}
func cloudDigest(typ, key, value string) string {
	return engineDigest("cloud:" + typ + ":" + key + ":" + value)
}
func (a *cloudEngineAPI) check(t *testing.T, cfg map[string]any, n int, tag string) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !reflect.DeepEqual(a.cfg, cfg) || a.mutations != n || !reflect.DeepEqual(a.tags, []string{tag}) {
		t.Fatalf("backend readback mismatch: config_equal=%t tags_equal=%t mutations=%d want=%d (values withheld)", reflect.DeepEqual(a.cfg, cfg), reflect.DeepEqual(a.tags, []string{tag}), a.mutations, n)
	}
}
func (a *cloudEngineAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fail := func(s string) { a.errors = append(a.errors, s); http.Error(w, s, 400) }
	if r.Header.Get("Authorization") != engineToken {
		fail("wrong fake token")
		return
	}
	if r.Method == "GET" && r.URL.Path == engineAPI+"read/"+engineID {
		a.reads++
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		cfg := cloudCopy(a.cfg)
		keys := []string{}
		digests := map[string]string{}
		for key, value := range a.cfg {
			if cloudSecret(a.typ, key) && value != "" {
				delete(cfg, key)
				keys = append(keys, key)
				digests[key] = cloudDigest(a.typ, key, value.(string))
			}
			if a.absentFalse && value == false {
				delete(cfg, key)
			}
		}
		sort.Strings(keys)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "name": "engine-resource", "integrationType": a.typ, "configuration": cfg, "userTags": a.tags, "redactedKeys": keys, "redactedDigests": digests})
		return
	}
	create := r.URL.Path == engineAPI+"create"
	update := r.URL.Path == engineAPI+"update/"+engineID
	if r.Method != "POST" || (!create && !update) {
		fail("unexpected endpoint")
		return
	}
	a.posts++
	if create == a.exists {
		fail("incorrect lifecycle")
		return
	}
	var dto struct {
		Name   string   `json:"name"`
		Type   string   `json:"integrationType"`
		Config string   `json:"config"`
		Tags   []string `json:"userTags"`
	}
	if json.NewDecoder(r.Body).Decode(&dto) != nil || dto.Type != a.typ || (create && dto.Name != "engine-resource") {
		fail("incorrect resource DTO")
		return
	}
	var cfg map[string]any
	if yaml.UnmarshalStrict([]byte(dto.Config), &cfg) != nil {
		fail("invalid configuration YAML")
		return
	}
	types := map[string]string{}
	if a.typ == "datadog" {
		types = map[string]string{"dd_api_key": "string", "dd_app_key": "string", "dd_site": "string"}
	} else {
		for _, key := range []string{"aws_region_name", "aws_access_key_id", "aws_secret_access_key", "aws_role_arn", "service_account", "version"} {
			types[key] = "string"
		}
		for _, key := range []string{"use_role_arn", "use_service_account", "create_if_not_exists"} {
			types[key] = "bool"
		}
		if a.typ == "aws" {
			types["enable_allowed_commands"], types["allowed_commands"] = "bool", "string"
		} else {
			types["keyspace"], types["keyspaces_endpoint"] = "string", "string"
		}
	}
	// name is legacy provider metadata ignored by the backend.
	if name, ok := cfg["name"]; ok {
		if name != "engine-resource" {
			fail("incorrect metadata name")
			return
		}
		delete(cfg, "name")
	}
	for key, value := range cfg {
		kind, ok := types[key]
		if !ok {
			fail("unknown configuration key " + key)
			return
		}
		if fmt.Sprintf("%T", value) != kind {
			fail("wrong configuration type for " + key)
			return
		}
	}
	for key := range types {
		if _, ok := cfg[key]; !ok {
			fail("full replacement missing key " + key)
			return
		}
	}
	// The expected full config is an independently specified backend fixture or
	// explicit scenario change, not a round trip through provider DTOs.
	if !reflect.DeepEqual(cfg, a.expected) {
		keys := []string{}
		for key, want := range a.expected {
			if !reflect.DeepEqual(cfg[key], want) {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		fail("replacement changed unexpected fields: " + strings.Join(keys, ", "))
		return
	}
	a.exists, a.cfg, a.tags = true, cloudCopy(cfg), dto.Tags
	a.mutations++
	_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
}
