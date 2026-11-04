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

// These fixtures are independently transcribed from inventorize 52bce48e:
// internal/service/integrations/{mysql,kafka,elasticache,mongodb,syslog,
// servicelist,sqlserver,google_oauth,ssh,juniper_sw,sophos_fw,remote_desktop},
// chrome/managed.go, handler/terraform_read.go and pkg/configexport/classify.go.
// They intentionally never call provider serialization or reverse mappings.
// This tests configuration transport/state only, not IAM tokens, TLS handshakes,
// browser automation, SSH/RDP sessions or Google credential safety in a live API.
// In particular serviceAccount is allow-listed by the backend exporter even for
// Google JSON credentials; this fake must NOT pretend the backend redacts it.
// Run: ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^TestIntegrationFieldsTerraformEngine$' -count=1 -v
// ADAPTIVE_ENGINE_PROVIDER_BINARY can select an independently built baseline.
func TestIntegrationFieldsTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform")
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
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline provider build failed: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine binary must be named terraform-provider-adaptive")
	}
	if st, err := os.Stat(bin); err != nil || st.IsDir() || st.Mode()&0111 == 0 {
		t.Fatal("engine binary missing or not executable")
	}
	for _, fixture := range integrationFieldFixtures() {
		f := fixture
		t.Run(f.typ+"/create_metadata_drift_ignore_reset_saved_plan", func(t *testing.T) {
			h := newIntegrationFieldsEngine(t, terraform, bin, f, false)
			attrs := integrationFieldsClone(f.attrs)
			h.config(attrs, "initial", "")
			h.run(0, "apply", "-auto-approve")
			h.check(f.wire, 1, "initial")
			h.clean()
			h.config(attrs, "changed", "")
			h.run(0, "apply", "-auto-approve")
			h.check(f.wire, 2, "changed")
			h.clean()
			// A different backend value must be observed in a saved real Terraform plan,
			// and applying that very plan must converge, not merely mutate local state.
			remote := integrationFieldsClone(f.wire)
			remote[f.driftWire] = f.driftValue
			h.api.seed(remote)
			h.api.expect(f.wire)
			h.run(2, "plan", "-detailed-exitcode", "-out=drift.tfplan")
			h.planUpdate("drift.tfplan", f.driftAttr, f.driftValue, attrs[f.driftAttr])
			h.run(0, "apply", "-auto-approve", "drift.tfplan")
			h.check(f.wire, 3, "changed")
			h.clean()
			// ignore_changes must preserve observed policy on a full replacement POST.
			h.api.seed(remote)
			h.api.expect(remote)
			h.config(attrs, "ignored", f.driftAttr)
			h.run(0, "apply", "-auto-approve")
			h.check(remote, 4, "ignored")
			h.clean()
			// Explicit false and empty strings reset policy instead of preserving it.
			cleared := integrationFieldsClone(f.wire)
			for attr, v := range f.reset {
				attrs[attr] = v
				cleared[f.names[attr]] = v
			}
			h.api.expect(cleared)
			h.config(attrs, "cleared", "")
			h.run(2, "plan", "-detailed-exitcode", "-out=reset.tfplan")
			h.run(0, "apply", "-auto-approve", "reset.tfplan")
			h.check(cleared, 5, "cleared")
			h.clean()
			if f.typ == "juniper_sw" || f.typ == "sophos_fw" {
				// An acknowledged clear must allow a later metadata update
				// without resupplying already-cleared browser credentials.
				for _, attr := range []string{"fields", "script"} {
					delete(attrs, attr)
					delete(cleared, f.names[attr])
				}
				h.api.expect(cleared)
				h.config(attrs, "omitted", "")
				h.run(0, "apply", "-auto-approve")
				h.check(cleared, 6, "omitted")
				h.clean()
			}
		})
		t.Run(f.typ+"/canonical_import_credential_adoption_clean_plan_metadata_preserve", func(t *testing.T) {
			h := newIntegrationFieldsEngine(t, terraform, bin, f, true)
			h.config(f.attrs, "initial", "")
			h.run(0, "import", "adaptive_resource.test", engineID)
			h.check(f.wire, 0, "initial")
			writes := 0
			if len(f.redacted) != 0 {
				// Import cannot reconstruct withheld plaintext, even when the user
				// has configured the same value. Explicitly adopt credentials before
				// requiring a clean plan; do not fake plaintext import responses.
				h.run(2, "plan", "-detailed-exitcode", "-out=adopt.tfplan")
				h.run(0, "apply", "-auto-approve", "adopt.tfplan")
				writes++
			}
			h.clean()
			h.check(f.wire, writes, "initial")
			h.config(f.attrs, "changed", "")
			h.run(0, "apply", "-auto-approve")
			h.check(f.wire, writes+1, "changed")
			h.clean()
		})
		guards := []string{}
		if f.guardAttr != "" {
			guards = append(guards, f.guardAttr)
		}
		if f.typ == "google" {
			// serviceAccount is credential material even on a legacy backend
			// that exposes it without fingerprint metadata. Ignoring its drift
			// must reject, not bypass the planned/configured-value guard.
			guards = append(guards, "service_account")
		}
		if f.typ == "juniper_sw" || f.typ == "sophos_fw" {
			guards = append(guards, "fields", "script")
		}
		for _, guard := range guards {
			t.Run(f.typ+"/opaque_import_omission_never_post/"+guard, func(t *testing.T) {
				h := newIntegrationFieldsEngine(t, terraform, bin, f, true)
				attrs := integrationFieldsClone(f.attrs)
				delete(attrs, guard)
				h.config(attrs, "initial", "")
				h.run(0, "import", "adaptive_resource.test", engineID)
				h.config(attrs, "changed", "")
				out := h.run(1, "apply", "-auto-approve")
				if !strings.Contains(out, "server-held") && !(f.typ == "rdp_windows" && strings.Contains(out, "requires a non-empty `password`")) {
					t.Fatalf("missing opaque-value guard diagnostic: %s", engineSafe(out))
				}
				h.check(f.wire, 0, "initial")
				h.noPosts(0)
			})
			t.Run(f.typ+"/opaque_rotation_ignore_changes_blocks_metadata_write/"+guard, func(t *testing.T) {
				h := newIntegrationFieldsEngine(t, terraform, bin, f, false)
				h.config(f.attrs, "initial", "")
				h.run(0, "apply", "-auto-approve")
				remote := integrationFieldsClone(f.wire)
				remote[f.names[guard]] = engineRotated
				h.api.seed(remote)
				h.run(0, "apply", "-refresh-only", "-auto-approve")
				h.config(f.attrs, "changed", guard)
				out := h.run(1, "apply", "-auto-approve")
				if !strings.Contains(out, "ignore_changes") {
					t.Fatalf("wrong ignored-credential diagnostic: %s", engineSafe(out))
				}
				h.check(remote, 1, "initial")
				h.noPosts(1)
			})
		}
	}
	// Pointer presence is semantically significant: absent useRdsIam allows
	// authMode fallback, whereas explicit false overrides legacy aws_rds_iam.
	for _, explicitFalse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mysql_pointer_omission_vs_false/%t", explicitFalse), func(t *testing.T) {
			f := integrationFieldFixtures()[0]
			// Isolate nil/false IAM semantics from unrelated MySQL SSL support.
			delete(f.attrs, "ssl_mode")
			f.wire["sslMode"] = "require"
			for attr, key := range map[string]string{"aws_region": "awsRegion", "aws_role_arn": "awsRoleArn", "aws_service_account": "awsServiceAccount"} {
				delete(f.attrs, attr)
				delete(f.wire, key)
			}
			delete(f.attrs, "use_rds_iam")
			delete(f.attrs, "use_irsa")
			delete(f.wire, "useRdsIam")
			delete(f.wire, "useIrsa")
			f.attrs["auth_mode"] = "aws_rds_iam"
			f.wire["authMode"] = "aws_rds_iam"
			if explicitFalse {
				f.attrs["use_rds_iam"] = false
				f.attrs["use_irsa"] = false
				f.wire["useRdsIam"] = false
				f.wire["useIrsa"] = false
			}
			h := newIntegrationFieldsEngine(t, terraform, bin, f, false)
			h.config(f.attrs, "initial", "")
			h.run(0, "apply", "-auto-approve")
			h.check(f.wire, 1, "initial")
			h.clean()
			h.config(f.attrs, "changed", "")
			h.run(0, "apply", "-auto-approve")
			h.check(f.wire, 2, "changed")
			h.clean()
		})
	}
	t.Log("Google serviceAccount read fixture intentionally remains clear: backend exporter allow-list is NOT a live credential-redaction guarantee")
}

type integrationFieldsFixture struct {
	typ, wireType        string
	attrs, wire          map[string]any
	names                map[string]string // Explicit public attribute -> backend key, not production metadata.
	driftAttr, driftWire string
	driftValue           any
	reset                map[string]any
	guardAttr            string
	redacted             []string
	defaults             map[string]any // Only these known backend fields may appear as empty defaults.
	pointers             []string
}

func integrationFieldFixtures() []integrationFieldsFixture {
	f := []integrationFieldsFixture{
		{typ: "mysql", attrs: map[string]any{"host": "db.invalid", "port": "3306", "username": "fixture-user", "database_name": "fixture-db", "ssl_mode": "verify_identity", "use_rds_iam": true, "use_irsa": false, "auth_mode": "aws_rds_iam", "aws_region": "us-east-2", "aws_role_arn": "arn:aws:iam::123456789012:role/fixture", "aws_service_account": "fixture-identity"},
			wire:  map[string]any{"hostname": "db.invalid", "port": "3306", "username": "fixture-user", "databaseName": "fixture-db", "sslMode": "verify_identity", "useRdsIam": true, "useIrsa": false, "authMode": "aws_rds_iam", "awsRegion": "us-east-2", "awsRoleArn": "arn:aws:iam::123456789012:role/fixture", "awsServiceAccount": "fixture-identity"},
			names: map[string]string{"use_rds_iam": "useRdsIam", "use_irsa": "useIrsa", "auth_mode": "authMode", "aws_service_account": "awsServiceAccount"}, driftAttr: "use_rds_iam", driftWire: "useRdsIam", driftValue: false, reset: map[string]any{"use_rds_iam": false, "use_irsa": false, "auth_mode": "", "aws_service_account": ""},
			defaults: map[string]any{"version": "", "password": "", "rootCert": "", "clientCert": "", "clientKey": "", "oldVersion": false, "awsAccessKeyId": "", "awsSecretAccessKey": ""}, pointers: []string{"useRdsIam", "useIrsa"}},
		{typ: "kafka", attrs: map[string]any{"client_configuration": "request.timeout.ms=30000", "use_msk_iam": true, "use_irsa": true, "bootstrap_servers": "b-1.fixture.kafka.us-east-2.amazonaws.com:9098,b-2.fixture.kafka.us-east-2.amazonaws.com:9098", "aws_region": "us-east-2", "aws_role_arn": "arn:aws:iam::123456789012:role/fixture", "aws_service_account": "fixture-identity"},
			wire:  map[string]any{"client_configuration": "request.timeout.ms=30000", "useMskIam": true, "useIrsa": true, "bootstrapServers": "b-1.fixture.kafka.us-east-2.amazonaws.com:9098,b-2.fixture.kafka.us-east-2.amazonaws.com:9098", "awsRegion": "us-east-2", "awsRoleArn": "arn:aws:iam::123456789012:role/fixture", "awsServiceAccount": "fixture-identity"},
			names: map[string]string{"use_msk_iam": "useMskIam", "bootstrap_servers": "bootstrapServers", "use_irsa": "useIrsa", "aws_service_account": "awsServiceAccount"}, driftAttr: "bootstrap_servers", driftWire: "bootstrapServers", driftValue: "b-other.fixture.kafka.us-east-2.amazonaws.com:9098", reset: map[string]any{"use_msk_iam": false, "use_irsa": false, "bootstrap_servers": "", "aws_service_account": ""}, defaults: map[string]any{"awsAccessKeyId": "", "awsSecretAccessKey": ""}, pointers: []string{"useMskIam", "useIrsa"}},
		{typ: "awselasticcache", attrs: map[string]any{"host": "cache.invalid", "port": "6379", "username": "fixture-user", "tls_enabled": true, "tls_skip_verify": true, "access_control_method": "user_group", "access_control_group": "fixture-group", "aws_region_name": "us-east-2", "use_iam_auth": true, "use_irsa": true, "aws_role_arn": "arn:aws:iam::123456789012:role/fixture", "aws_service_account": "fixture-identity", "cache_name": "fixture-cache", "cache_type": "serverless"},
			wire:  map[string]any{"host": "cache.invalid", "port": "6379", "username": "fixture-user", "tlsEnabled": true, "tlsSkipVerify": true, "access_control_method": "user_group", "access_control_group": "fixture-group", "aws_region_name": "us-east-2", "useIamAuth": true, "useIrsa": true, "aws_role_arn": "arn:aws:iam::123456789012:role/fixture", "aws_service_account": "fixture-identity", "cache_name": "fixture-cache", "cache_type": "serverless"},
			names: map[string]string{"use_iam_auth": "useIamAuth", "use_irsa": "useIrsa", "cache_name": "cache_name", "cache_type": "cache_type", "tls_skip_verify": "tlsSkipVerify"}, driftAttr: "cache_name", driftWire: "cache_name", driftValue: "other-cache", reset: map[string]any{"use_iam_auth": false, "use_irsa": false, "cache_name": "", "cache_type": "", "tls_skip_verify": false}, defaults: map[string]any{"version": "1.0", "password": "", "aws_access_key_id": "", "aws_secret_access_key": ""}, pointers: []string{"useIamAuth", "useIrsa"}},
		{typ: "mongodb", attrs: map[string]any{"uri": "mongodb://db.invalid/fixture", "use_tls": true, "client_certificate": enginePassword}, wire: map[string]any{"uri": "mongodb://db.invalid/fixture", "useTLS": true, "clientCertificate": enginePassword},
			names: map[string]string{"use_tls": "useTLS", "client_certificate": "clientCertificate"}, driftAttr: "use_tls", driftWire: "useTLS", driftValue: false, reset: map[string]any{"use_tls": false, "client_certificate": ""}, guardAttr: "client_certificate", redacted: []string{"uri", "clientCertificate"}, defaults: map[string]any{"version": ""}},
		{typ: "syslog", attrs: map[string]any{"hostname": "logs.invalid", "port": "6514", "protocol": "tcp", "tls_enabled": true, "tls_skip_verify": true, "tls_ca_cert": "fixture-public-ca", "client_certificate": enginePassword, "client_key": enginePassword},
			wire:  map[string]any{"hostname": "logs.invalid", "port": "6514", "protocol": "tcp", "tlsEnabled": true, "insecureSkipVerify": true, "caCertificate": "fixture-public-ca", "clientCertificate": enginePassword, "clientKey": enginePassword},
			names: map[string]string{"tls_enabled": "tlsEnabled", "tls_skip_verify": "insecureSkipVerify", "tls_ca_cert": "caCertificate", "client_certificate": "clientCertificate", "client_key": "clientKey"}, driftAttr: "tls_enabled", driftWire: "tlsEnabled", driftValue: false, reset: map[string]any{"tls_enabled": false, "tls_skip_verify": false, "client_certificate": "", "client_key": ""}, guardAttr: "client_key", redacted: []string{"clientCertificate", "clientKey"}},
		{typ: "services", wireType: "servicelist", attrs: map[string]any{"urls": "https://service.invalid", "tls_enabled": true}, wire: map[string]any{"urls": "https://service.invalid", "enableTLS": true}, names: map[string]string{"tls_enabled": "enableTLS"}, driftAttr: "tls_enabled", driftWire: "enableTLS", driftValue: false, reset: map[string]any{"tls_enabled": false}, defaults: map[string]any{"version": "1"}},
		{typ: "sql_server", attrs: map[string]any{"host": "db.invalid", "port": "1433", "username": "fixture-user", "database_name": "fixture-db", "password": enginePassword, "ssl_mode": "verify-full"}, wire: map[string]any{"hostname": "db.invalid", "port": "1433", "username": "fixture-user", "databaseName": "fixture-db", "password": enginePassword, "sslMode": "verify-full"}, names: map[string]string{"ssl_mode": "sslMode", "password": "password"}, driftAttr: "ssl_mode", driftWire: "sslMode", driftValue: "disable", reset: map[string]any{"ssl_mode": "require"}, guardAttr: "password", redacted: []string{"password"}, defaults: map[string]any{"version": ""}},
		{typ: "google", attrs: map[string]any{"domain": "accounts.google.com", "client_id": "fixture-client", "client_secret": enginePassword, "admin_email": "fixture-admin@example.invalid", "service_account": enginePassword}, wire: map[string]any{"domain": "accounts.google.com", "clientID": "fixture-client", "clientSecret": enginePassword, "adminEmail": "fixture-admin@example.invalid", "serviceAccount": enginePassword}, names: map[string]string{"admin_email": "adminEmail", "service_account": "serviceAccount", "client_secret": "clientSecret"}, driftAttr: "domain", driftWire: "domain", driftValue: "drift.example.invalid", reset: map[string]any{"admin_email": "", "service_account": ""}, guardAttr: "client_secret", redacted: []string{"adminEmail", "clientSecret"}, defaults: map[string]any{"version": "1"}},
		{typ: "rdp_windows", attrs: map[string]any{"hostname": "desktop.invalid", "port": "3389", "username": "fixture-user", "password": enginePassword, "allow_file_transfer": true}, wire: map[string]any{"hostname": "desktop.invalid", "port": "3389", "username": "fixture-user", "password": enginePassword, "allowFileTransfer": true}, names: map[string]string{"allow_file_transfer": "allowFileTransfer", "password": "password"}, driftAttr: "allow_file_transfer", driftWire: "allowFileTransfer", driftValue: false, reset: map[string]any{"allow_file_transfer": false}, guardAttr: "password", redacted: []string{"password"}, defaults: map[string]any{"version": "1.0"}},
	}
	for _, typ := range []string{"juniper_sw", "sophos_fw"} {
		f = append(f, integrationFieldsFixture{typ: typ, attrs: map[string]any{"hostname": "device.invalid", "port": "22", "username": "fixture-user", "key": enginePassword, "webui_port": "8443", "login_url": "/fixture-login", "automation_mode": "fields", "fields": "#password => " + enginePassword, "script": enginePassword},
			wire:  map[string]any{"hostname": "device.invalid", "port": "22", "username": "fixture-user", "sshKey": enginePassword, "usePassword": false, "webui_port": "8443", "login_url": "/fixture-login", "automationMode": "fields", "fields": "#password => " + enginePassword, "script": enginePassword},
			names: map[string]string{"key": "sshKey", "webui_port": "webui_port", "login_url": "login_url", "automation_mode": "automationMode", "fields": "fields", "script": "script"}, driftAttr: "automation_mode", driftWire: "automationMode", driftValue: "none", reset: map[string]any{"login_url": "", "automation_mode": "none", "fields": "", "script": ""}, guardAttr: "key", redacted: []string{"sshKey", "fields", "script"}, defaults: map[string]any{"version": "1.0", "password": ""}})
	}
	for i := range f {
		if f[i].wireType == "" {
			f[i].wireType = f[i].typ
		}
	}
	return f
}

type integrationFieldsEngine struct {
	*secretEngine
	api *integrationFieldsAPI
}

func newIntegrationFieldsEngine(t *testing.T, terraform, bin string, f integrationFieldsFixture, existing bool) *integrationFieldsEngine {
	t.Helper()
	a := &integrationFieldsAPI{f: f, exists: existing, current: integrationFieldsClone(f.wire), expected: integrationFieldsClone(f.wire), tag: "initial"}
	server := httptest.NewServer(a)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(dir, "terraform.rc")
	engineWrite(t, cli, fmt.Sprintf("provider_installation {\n dev_overrides {\n \"adaptive-scale/adaptive\" = %q\n }\n}\ndisable_checkpoint = true\n", filepath.Dir(bin)))
	h := &integrationFieldsEngine{secretEngine: &secretEngine{t: t, terraform: terraform, dir: dir, url: server.URL, env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + os.Getenv("TMPDIR"), "TF_CLI_CONFIG_FILE=" + cli, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1"}}, api: a}
	t.Cleanup(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.errors) > 0 {
			t.Errorf("strict backend contract violations (values withheld): %s", strings.Join(a.errors, "; "))
		}
	})
	return h
}
func (h *integrationFieldsEngine) config(attrs map[string]any, tag, ignore string) {
	h.t.Helper()
	var body strings.Builder
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		v, _ := json.Marshal(attrs[key])
		fmt.Fprintf(&body, " %s = %s\n", key, v)
	}
	if ignore != "" {
		fmt.Fprintf(&body, " lifecycle { ignore_changes = [%s] }\n", ignore)
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
 type = %q
 name = "engine-resource"
 tags = [%q]
%s
}
`, engineToken, h.url, h.api.f.typ, tag, body.String()))
}
func (h *integrationFieldsEngine) run(want int, args ...string) string {
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
			h.t.Fatal("Terraform launch failed")
		}
	}
	if code != want {
		safe := engineSafe(string(out))
		for _, key := range h.api.f.redacted {
			for _, value := range []string{enginePassword, engineRotated, fmt.Sprint(h.api.f.wire[key])} {
				safe = strings.ReplaceAll(safe, cloudDigest(h.api.f.wireType, key, value), "[REDACTED-TEST-DIGEST]")
			}
		}
		h.t.Fatalf("terraform %s: exit %d want %d\n%s", strings.Join(args, " "), code, want, safe)
	}
	h.t.Logf("terraform %s: exit %d", strings.Join(args, " "), code)
	return string(out)
}
func (h *integrationFieldsEngine) clean() { h.t.Helper(); h.run(0, "plan", "-detailed-exitcode") }
func (h *integrationFieldsEngine) check(wire map[string]any, n int, tag string) {
	h.t.Helper()
	h.api.mu.Lock()
	defer h.api.mu.Unlock()
	if !reflect.DeepEqual(h.api.current, wire) || h.api.mutations != n || h.api.tag != tag {
		h.t.Fatalf("canonical backend state mismatch (values withheld); mutations=%d want=%d tag_matches=%t", h.api.mutations, n, h.api.tag == tag)
	}
}
func (h *integrationFieldsEngine) noPosts(n int) {
	h.t.Helper()
	h.api.mu.Lock()
	defer h.api.mu.Unlock()
	if h.api.posts != n {
		h.t.Fatalf("credential guard reached POST: got %d want %d", h.api.posts, n)
	}
}
func (h *integrationFieldsEngine) planUpdate(file, attr string, before, after any) {
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
	if json.Unmarshal([]byte(h.run(0, "show", "-json", file)), &p) != nil {
		h.t.Fatal("invalid actual Terraform plan JSON")
	}
	for _, c := range p.Changes {
		if c.Address == "adaptive_resource.test" && reflect.DeepEqual(c.Change.Actions, []string{"update"}) && reflect.DeepEqual(c.Change.Before[attr], before) && reflect.DeepEqual(c.Change.After[attr], after) {
			return
		}
	}
	h.t.Fatalf("saved plan does not restore configured %s (values withheld)", attr)
}

type integrationFieldsAPI struct {
	mu                sync.Mutex
	f                 integrationFieldsFixture
	exists            bool
	current, expected map[string]any
	tag               string
	posts, mutations  int
	errors            []string
}

func (a *integrationFieldsAPI) seed(cfg map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.current = integrationFieldsClone(cfg)
}
func (a *integrationFieldsAPI) expect(cfg map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expected = integrationFieldsClone(cfg)
}
func (a *integrationFieldsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) {
		a.errors = append(a.errors, message)
		http.Error(w, message, http.StatusBadRequest)
	}
	if r.Header.Get("Authorization") != engineToken {
		fail("missing explicit synthetic token")
		return
	}
	if r.Method == "GET" && r.URL.Path == engineAPI+"read/"+engineID {
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		cfg := integrationFieldsClone(a.current)
		delete(cfg, "name")
		redacted := []string{}
		digests := map[string]string{}
		for _, key := range a.f.redacted {
			if value, ok := cfg[key].(string); ok && value != "" {
				redacted = append(redacted, key)
				digests[key] = cloudDigest(a.f.wireType, key, value)
				delete(cfg, key)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID, "name": "engine-resource", "integrationType": a.f.wireType, "configuration": cfg, "userTags": []string{a.tag}, "redactedKeys": redacted, "redactedDigests": digests})
		return
	}
	create := r.URL.Path == engineAPI+"create"
	update := r.URL.Path == engineAPI+"update/"+engineID
	if r.Method != "POST" || (!create && !update) {
		fail("unexpected method or endpoint")
		return
	}
	a.posts++
	if create == a.exists {
		fail("invalid create/update lifecycle")
		return
	}
	var req struct {
		Name           string   `json:"name"`
		Type           string   `json:"integrationType"`
		Config         string   `json:"config"`
		Tags           []string `json:"userTags"`
		DefaultCluster string   `json:"defaultCluster"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || req.Type != a.f.wireType || (create && req.Name != "engine-resource") || len(req.Tags) != 1 || req.DefaultCluster != "" {
		fail("invalid exact write DTO or integration wire type")
		return
	}
	var cfg map[string]any
	if yaml.UnmarshalStrict([]byte(req.Config), &cfg) != nil {
		fail("invalid or duplicate-key YAML configuration")
		return
	}
	// Never accept arbitrary echoed fields. Every non-empty field must match an
	// independently seeded expectation, including its concrete YAML scalar type.
	// Empty backend struct defaults are allowed only on the explicit allow-list.
	for key, value := range cfg {
		if key == "name" {
			if value != "engine-resource" {
				fail("invalid configuration name")
				return
			}
			continue
		}
		expected, ok := a.expected[key]
		if !ok {
			expected, ok = a.f.defaults[key]
		}
		if !ok || !reflect.DeepEqual(value, expected) {
			fail("unexpected key, scalar type or value for " + key)
			return
		}
	}
	for key, value := range a.expected {
		actual, ok := cfg[key]
		if !ok {
			pointer := false
			for _, k := range a.f.pointers {
				pointer = pointer || k == key
			}
			if !pointer && key != "fields" && key != "script" && (value == "" || value == false) {
				continue
			} // ordinary omitempty reset
		}
		if !ok || !reflect.DeepEqual(actual, value) {
			fail("missing or changed canonical field " + key)
			return
		}
	}
	// Omitted pointer switches must stay absent, not become explicit false.
	for _, key := range a.f.pointers {
		if _, wanted := a.expected[key]; !wanted {
			if _, present := cfg[key]; present {
				fail("omitted pointer emitted as explicit value: " + key)
				return
			}
		}
	}
	a.exists = true
	a.current = integrationFieldsClone(a.expected)
	a.tag = req.Tags[0]
	a.mutations++
	_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
}
func integrationFieldsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
