package components

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// Independent backend-shaped decoder: do not use the provider's wire struct.
type arubaBackendContract struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Hostname    string `yaml:"hostname"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	SSHKey      string `yaml:"sshKey"`
	UsePassword bool   `yaml:"usePassword"`
	Port        string `yaml:"port"`
	WebUIPort   string `yaml:"webui_port"`
	LoginURL    string `yaml:"login_url"`
}

func arubaOwnedData(t *testing.T, config map[string]cty.Value) *schema.ResourceData {
	t.Helper()
	r := ResourceAdaptiveResource()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"name": "switch", "type": "aruba_instant_on", "host": "stale-host", "hostname": "stale-hostname",
		"username": "old-user", "password": "held-password", "key": "held-key", "port": "22", "webui_port": "443", "login_url": "/old",
	})
	d.SetId("aruba-1")
	if config == nil {
		return d
	}
	state := d.State()
	state.RawConfig = cty.ObjectVal(config)
	return r.Data(state)
}

func TestArubaAliasOwnership(t *testing.T) {
	for _, tc := range []struct {
		name           string
		config         map[string]cty.Value
		host, hostname string
	}{
		{"host", map[string]cty.Value{"host": cty.StringVal("configured"), "hostname": cty.NullVal(cty.String)}, "remote", ""},
		{"hostname", map[string]cty.Value{"host": cty.NullVal(cty.String), "hostname": cty.StringVal("configured")}, "", "remote"},
		{"hostname-only-raw", map[string]cty.Value{"hostname": cty.StringVal("configured")}, "", "remote"},
		{"hostname-explicit-empty", map[string]cty.Value{"host": cty.NullVal(cty.String), "hostname": cty.StringVal("")}, "", "remote"},
		{"both-equal", map[string]cty.Value{"host": cty.StringVal("configured"), "hostname": cty.StringVal("configured")}, "remote", "remote"},
		{"empty-host-alias", map[string]cty.Value{"host": cty.StringVal(""), "hostname": cty.StringVal("configured")}, "", "remote"},
		{"import", nil, "remote", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := arubaOwnedData(t, tc.config)
			if err := integrations.ApplyIntegrationConfig(d, "aruba_instant_on", map[string]interface{}{"hostname": "remote", "host": "legacy"}); err != nil {
				t.Fatal(err)
			}
			if d.Get("host") != tc.host || d.Get("hostname") != tc.hostname {
				t.Errorf("alias ownership: host=%q hostname=%q", d.Get("host"), d.Get("hostname"))
			}
			built := integrations.SchemaToArubaInstantOnIntegrationConfiguration(d)
			wire, err := yaml.Marshal(built)
			if err != nil {
				t.Fatal(err)
			}
			var decoded arubaBackendContract
			if err := yaml.UnmarshalStrict(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Hostname != "remote" {
				t.Error("refresh/build lost canonical hostname")
			}
			// A second refresh must not populate an unconfigured alias.
			if err := integrations.ApplyIntegrationConfig(d, "aruba_instant_on", map[string]interface{}{"hostname": "remote"}); err != nil {
				t.Fatal(err)
			}
			if d.Get("host") != tc.host || d.Get("hostname") != tc.hostname {
				t.Error("alias state is not stable across refresh")
			}
		})
	}
}

func TestArubaRefreshCredentialsAndOptionalClears(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cfg           map[string]interface{}
		password, key string
	}{
		{"withheld", map[string]interface{}{"hostname": "remote"}, "held-password", "held-key"},
		{"nil", map[string]interface{}{"password": nil, "sshKey": nil}, "held-password", "held-key"},
		{"empty", map[string]interface{}{"password": "", "sshKey": ""}, "", ""},
		{"rotated", map[string]interface{}{"password": "new-password", "sshKey": "new-key"}, "new-password", "new-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := arubaOwnedData(t, nil)
			tc.cfg["webui_port"], tc.cfg["login_url"] = "", ""
			if err := integrations.ApplyIntegrationConfig(d, "aruba_instant_on", tc.cfg); err != nil {
				t.Fatal(err)
			}
			if d.Get("password") != tc.password || d.Get("key") != tc.key {
				t.Error("credentials did not preserve withheld/null or apply explicit empty/updated values")
			}
			if d.Get("webui_port") != "" || d.Get("login_url") != "" {
				t.Error("optional empty values did not clear state")
			}
		})
	}
	for wire, attr := range map[string]string{"password": "password", "sshKey": "key"} {
		if got, ok := integrations.AttrForConfigKey("aruba_instant_on", wire); !ok || got != attr {
			t.Errorf("fingerprint mapping %s does not resolve to %s", wire, attr)
		}
	}
	if _, ok := integrations.AttrForConfigKey("aruba_instant_on", "apiToken"); ok {
		t.Error("obsolete unsupported API token still mapped")
	}
}

func TestArubaHostnamePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  map[string]interface{}
		want string
	}{
		{"legacy", map[string]interface{}{"host": "legacy"}, "legacy"},
		{"canonical", map[string]interface{}{"host": "legacy", "hostname": "canonical"}, "canonical"},
		{"canonical-empty", map[string]interface{}{"host": "legacy", "hostname": ""}, ""},
		{"canonical-null", map[string]interface{}{"host": "legacy", "hostname": nil}, "stale-host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := arubaOwnedData(t, nil)
			if err := integrations.ApplyIntegrationConfig(d, "aruba_instant_on", tc.cfg); err != nil {
				t.Fatal(err)
			}
			if d.Get("host") != tc.want {
				t.Errorf("hostname precedence: got %q want %q", d.Get("host"), tc.want)
			}
		})
	}
}

func TestArubaBuilderUsesConfiguredAlias(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		config               map[string]cty.Value
		host, hostname, want string
	}{
		{"host-owned-empty", map[string]cty.Value{"host": cty.StringVal(""), "hostname": cty.NullVal(cty.String)}, "", "stale-hostname", ""},
		{"hostname-owned", map[string]cty.Value{"host": cty.NullVal(cty.String), "hostname": cty.StringVal("configured")}, "stale-host", "configured", "configured"},
		{"legacy-host", nil, "configured", "", "configured"},
		{"hostname-without-raw", nil, "", "configured", "configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := arubaOwnedData(t, tc.config)
			if err := d.Set("host", tc.host); err != nil {
				t.Fatal(err)
			}
			if err := d.Set("hostname", tc.hostname); err != nil {
				t.Fatal(err)
			}
			wire, err := yaml.Marshal(integrations.SchemaToArubaInstantOnIntegrationConfiguration(d))
			if err != nil {
				t.Fatal(err)
			}
			var decoded arubaBackendContract
			if err := yaml.UnmarshalStrict(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Hostname != tc.want {
				t.Error("builder selected stale or unconfigured alias")
			}
		})
	}
}

func TestArubaInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name, host, hostname, token string
		invalid                     bool
	}{
		{"host", "switch", "", "", false},
		{"hostname", "", "switch", "", false},
		{"equal-aliases", "switch", "switch", "", false},
		{"conflict", "sensitive-host-one", "sensitive-host-two", "", true},
		{"unsupported-token", "switch", "", "sensitive-token", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := integrations.ValidateArubaInstantOnInputs(tc.host, tc.hostname, tc.token)
			if (err != nil) != tc.invalid {
				t.Fatalf("validation invalid=%v, want %v", err != nil, tc.invalid)
			}
			if err != nil {
				for _, secret := range []string{"sensitive-host-one", "sensitive-host-two", "sensitive-token"} {
					if strings.Contains(err.Error(), secret) {
						t.Error("validation exposed input")
					}
				}
				if tc.token != "" && (!strings.Contains(err.Error(), "api_token") || !strings.Contains(err.Error(), "remove")) {
					t.Error("unsupported token diagnostic is not actionable")
				}
			}
		})
	}
}

func TestArubaCanonicalWire(t *testing.T) {
	for _, key := range []string{"", "synthetic-key"} {
		d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
			"name": "switch", "type": "aruba_instant_on", "host": "switch.example", "username": "operator",
			"password": "synthetic-password", "key": key, "port": "22", "webui_port": "8443", "login_url": "/login",
		})
		obj, err := schemaToResourceIntegrationConfiguration(d, "aruba_instant_on")
		if err != nil {
			t.Fatal(err)
		}
		wire, err := yaml.Marshal(obj)
		if err != nil {
			t.Fatal(err)
		}
		var decoded arubaBackendContract
		if err := yaml.UnmarshalStrict(wire, &decoded); err != nil {
			t.Fatalf("backend cannot decode canonical YAML: %v", err)
		}
		want := arubaBackendContract{"1.0", "switch", "switch.example", "operator", "synthetic-password", key, key == "", "22", "8443", "/login"}
		if !reflect.DeepEqual(decoded, want) {
			t.Error("canonical backend fields do not match input")
		}
		var got, golden map[string]interface{}
		if err := yaml.Unmarshal(wire, &got); err != nil {
			t.Fatal(err)
		}
		fixture := "version: '1.0'\nname: switch\nhostname: switch.example\nusername: operator\npassword: synthetic-password\nsshKey: ''\nusePassword: true\nport: '22'\nwebui_port: '8443'\nlogin_url: /login\n"
		if err := yaml.Unmarshal([]byte(fixture), &golden); err != nil {
			t.Fatal(err)
		}
		golden["sshKey"], golden["usePassword"] = key, key == ""
		if !reflect.DeepEqual(got, golden) {
			t.Error("wire does not match independent canonical golden")
		}
	}
}
