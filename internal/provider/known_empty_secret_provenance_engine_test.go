package provider_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestKnownEmptySecretTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("requires ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if binary == "" {
		t.Fatal("set ADAPTIVE_ENGINE_PROVIDER_BINARY to a separately built provider")
	}
	for _, mode := range []string{"first_addition", "ignore_changes", "legacy_unknown", "last_digest_clear", "explicit_resupply"} {
		t.Run(mode, func(t *testing.T) {
			h := newSecretEngine(t, terraform, binary, true)
			empty := ""
			h.api.mu.Lock()
			h.api.password = ""
			h.api.mu.Unlock()
			if mode == "last_digest_clear" {
				h.api.mu.Lock()
				h.api.password = enginePassword
				h.api.mu.Unlock()
			}
			ignored := []string{}
			if mode == "ignore_changes" {
				ignored = []string{"password"}
			}
			h.config(&empty, "initial", ignored...)
			h.run(0, "import", "adaptive_resource.test", engineID)
			if mode == "last_digest_clear" {
				h.config(&empty, "changed")
				h.run(0, "apply", "-auto-approve")
				h.config(&empty, "initial")
				h.run(0, "apply", "-auto-approve")
				if len(h.state().Digests) != 0 {
					t.Fatal("last digest did not clear")
				}
			}
			if mode == "legacy_unknown" {
				path := filepath.Join(h.dir, "terraform.tfstate")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var state map[string]any
				if json.Unmarshal(raw, &state) != nil {
					t.Fatal("invalid state")
				}
				for _, r := range state["resources"].([]any) {
					for _, i := range r.(map[string]any)["instances"].([]any) {
						delete(i.(map[string]any)["attributes"].(map[string]any), "known_empty_secret_keys")
					}
				}
				rewritten, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				engineWrite(t, path, string(rewritten))
			}
			before := h.api.mutationCount()
			h.api.mu.Lock()
			h.api.password = engineRotated
			h.api.mu.Unlock()
			for cycle := 0; cycle < 2; cycle++ {
				h.run(0, "apply", "-refresh-only", "-auto-approve")
				state := h.state()
				if mode == "legacy_unknown" {
					if state.Digests["password"] != engineDigest(engineRotated) {
						t.Fatal("legacy unknown did not seed")
					}
				} else {
					if state.Password != engineDigest(engineRotated) || len(state.Digests) != 0 {
						t.Fatal("first addition was seeded away instead of marked")
					}
				}
			}
			if mode == "legacy_unknown" {
				return
			}
			if mode == "ignore_changes" {
				h.run(0, "plan", "-detailed-exitcode")
				h.config(&empty, "changed", "password")
				out := h.run(1, "apply", "-auto-approve")
				if !strings.Contains(out, "ignore_changes") {
					t.Fatal("ignored marker failed for unrelated reason")
				}
				h.api.assert(t, engineRotated, []string{"initial"}, before)
				return
			}
			if mode == "explicit_resupply" {
				password := enginePassword
				h.config(&password, "initial")
				h.run(0, "apply", "-auto-approve")
				h.api.assert(t, enginePassword, []string{"initial"}, before+1)
				if h.state().Digests["password"] != engineDigest(enginePassword) {
					t.Fatal("explicit resupply did not establish baseline")
				}
				raw, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
				if err != nil {
					t.Fatal(err)
				}
				var state struct {
					Resources []struct {
						Instances []struct {
							Attributes struct {
								Known []string `json:"known_empty_secret_keys"`
							} `json:"attributes"`
						} `json:"instances"`
					} `json:"resources"`
				}
				if err := json.Unmarshal(raw, &state); err != nil {
					t.Fatalf("cannot decode state: %v", err)
				}
				if len(state.Resources[0].Instances[0].Attributes.Known) != 0 {
					t.Fatalf("verified resupply did not remove empty provenance: %v", state.Resources[0].Instances[0].Attributes.Known)
				}
				h.run(0, "plan", "-detailed-exitcode")
				return
			}
			h.run(2, "plan", "-detailed-exitcode", "-out=clear.tfplan")
			h.run(0, "apply", "-auto-approve", "clear.tfplan")
			h.api.assert(t, "", []string{"initial"}, before+1)
			h.run(0, "plan", "-detailed-exitcode")
		})
	}
}

// Failures are injected only into the loopback API. No live resources are used.
func TestKnownEmptySecretFailureTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("requires ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if binary == "" {
		t.Fatal("set ADAPTIVE_ENGINE_PROVIDER_BINARY")
	}
	for _, mode := range []string{"bad_read", "failed_post", "postwrite_read_failure", "partial_clear"} {
		t.Run(mode, func(t *testing.T) {
			h := newSecretEngine(t, terraform, binary, true)
			var failRead, failPost, failAfterPost atomic.Bool
			partial := mode == "partial_clear"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && failRead.Load() {
					http.Error(w, "synthetic read failure", 500)
					return
				}
				if r.Method == http.MethodPost && failPost.Load() {
					http.Error(w, "synthetic write failure", 500)
					return
				}
				rec := httptest.NewRecorder()
				h.api.ServeHTTP(rec, r)
				if partial && r.Method == http.MethodGet && rec.Code == 200 {
					var obj map[string]any
					if json.Unmarshal(rec.Body.Bytes(), &obj) != nil {
						http.Error(w, "bad fixture", 500)
						return
					}
					obj["redactedDigests"].(map[string]any)["clientKey"] = engineDigest(enginePassword)
					obj["redactedKeys"] = append(obj["redactedKeys"].([]any), "clientKey")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(obj)
				} else {
					w.WriteHeader(rec.Code)
					_, _ = w.Write(rec.Body.Bytes())
				}
				if r.Method == http.MethodPost && failAfterPost.Load() {
					failRead.Store(true)
				}
			}))
			defer server.Close()
			h.url = server.URL
			config := func(tag string) {
				empty := ""
				h.config(&empty, tag)
				if partial {
					p := filepath.Join(h.dir, "main.tf")
					raw, err := os.ReadFile(p)
					if err != nil {
						t.Fatal(err)
					}
					engineWrite(t, p, strings.Replace(string(raw), "password = ", "client_key = \""+enginePassword+"\"\n  password = ", 1))
				}
			}
			h.api.mu.Lock()
			if !partial {
				h.api.password = ""
			}
			h.api.mu.Unlock()
			config("initial")
			h.run(0, "import", "adaptive_resource.test", engineID)
			if partial {
				config("changed")
				h.run(0, "apply", "-auto-approve")
				if ds := h.state().Digests; len(ds) != 1 || ds["clientKey"] != engineDigest(enginePassword) {
					t.Fatal("partial clear did not preserve unaffected digest")
				}
				h.api.mu.Lock()
				h.api.password = engineRotated
				h.api.mu.Unlock()
				for i := 0; i < 2; i++ {
					h.run(0, "apply", "-refresh-only", "-auto-approve")
					if h.state().Password != engineDigest(engineRotated) || len(h.state().Digests) != 1 {
						t.Fatal("partial-clear first addition lost evidence")
					}
				}
				return
			}
			attrs := func() map[string]any {
				raw, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
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
				if json.Unmarshal(raw, &state) != nil {
					t.Fatal("invalid state")
				}
				return state.Resources[0].Instances[0].Attributes
			}
			baseline := attrs()
			if mode == "bad_read" {
				failRead.Store(true)
				h.run(1, "apply", "-refresh-only", "-auto-approve")
				if !reflect.DeepEqual(baseline, attrs()) {
					t.Fatal("failed read changed baseline")
				}
				return
			}
			h.api.mu.Lock()
			h.api.password = engineRotated
			h.api.mu.Unlock()
			h.run(0, "apply", "-refresh-only", "-auto-approve")
			if mode == "failed_post" {
				failPost.Store(true)
			} else {
				failAfterPost.Store(true)
			}
			code := 0
			if mode == "failed_post" {
				code = 1
			}
			h.run(code, "apply", "-auto-approve")
			current := attrs()
			if !reflect.DeepEqual(current["known_empty_secret_keys"], baseline["known_empty_secret_keys"]) || !reflect.DeepEqual(current["applied_digests"], baseline["applied_digests"]) {
				t.Fatal("failed write/readback advanced secret evidence")
			}
			want := 0
			if mode == "postwrite_read_failure" {
				want = 1
			}
			if h.api.mutationCount() != want {
				t.Fatal("unexpected mutation count")
			}
		})
	}
}
