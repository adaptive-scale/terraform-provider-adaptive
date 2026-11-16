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
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

// Exercises the old-backend response shape that exposed a credential but omitted
// both held-key and fingerprint metadata. This is an isolated API, not live auth.
func TestUnclassifiedSecretTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform")
	}
	binary := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if binary == "" {
		binary = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("provider build: %v\n%s", err, engineSafe(string(out)))
		}
	}
	binary, err = filepath.Abs(binary)
	if err != nil || filepath.Base(binary) != "terraform-provider-adaptive" {
		t.Fatal("invalid provider binary path")
	}
	h := newSecretEngine(t, terraform, binary, false)
	var mu sync.Mutex
	secret := enginePassword
	tags := []string{"initial"}
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != engineToken {
			http.Error(w, "wrong fixture token", 403)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == engineAPI+"read/"+engineID:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "name": "engine-resource", "integrationType": "1password", "userTags": tags,
				"configuration": map[string]any{"name": "engine-resource", "service_account": secret, "use_connect_server": false, "connect_server_url": ""}})
		case r.Method == http.MethodPost && r.URL.Path == engineAPI+"update/"+engineID:
			var request struct {
				Type   string   `json:"integrationType"`
				Config string   `json:"config"`
				Tags   []string `json:"userTags"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil || request.Type != "1password" {
				http.Error(w, "wrong fixture request", 400)
				return
			}
			var configuration map[string]any
			if yaml.Unmarshal([]byte(request.Config), &configuration) != nil {
				http.Error(w, "invalid fixture YAML", 400)
				return
			}
			value, ok := configuration["service_account"].(string)
			if !ok {
				http.Error(w, "missing fixture credential field", 400)
				return
			}
			secret, tags = value, request.Tags
			writes++
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.Error(w, "unexpected fixture route", 400)
		}
	}))
	defer server.Close()
	h.url = server.URL
	config := func(input, tag string) {
		engineWrite(t, filepath.Join(h.dir, "main.tf"), fmt.Sprintf(`terraform {
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
 type = "1password"
 tags = [%q]
 %s
}
`, engineToken, h.url, tag, input))
	}
	assert := func(expected string, count int) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if secret != expected || writes != count {
			t.Fatal("actual fixture credential/mutation count changed unexpectedly")
		}
	}
	config("", "initial")
	h.run(0, "import", "adaptive_resource.test", engineID)
	for _, input := range []string{"", "service_account = null"} {
		config(input, "changed")
		h.run(2, "plan", "-detailed-exitcode", "-out=omission.tfplan")
		output := h.run(1, "apply", "-auto-approve", "omission.tfplan")
		if !strings.Contains(output, "server-held values for service_account") {
			t.Fatal("credential omission failed for an unrelated reason")
		}
		assert(enginePassword, 0)
	}
	config(fmt.Sprintf("service_account = %q", enginePassword), "restored")
	h.run(0, "apply", "-auto-approve")
	assert(enginePassword, 1)
	h.run(0, "plan", "-detailed-exitcode")
	config(`service_account = ""`, "cleared")
	h.run(0, "apply", "-auto-approve")
	assert("", 2)
	config("", "after-clear")
	h.run(0, "apply", "-auto-approve")
	assert("", 3)
	h.run(0, "plan", "-detailed-exitcode")
}
