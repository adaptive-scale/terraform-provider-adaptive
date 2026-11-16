package components

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func TestRedshiftSSLWireContract(t *testing.T) {
	resource := ResourceAdaptiveResource()
	ssl := resource.Schema["ssl_mode"]
	if ssl.ValidateFunc == nil {
		t.Fatal("ssl_mode must retain its legacy validator")
	}
	for _, mode := range []string{"", "prefer", "allow", "require", "verify-ca", "verify-full", "disable"} {
		t.Run(fmt.Sprintf("mode=%q", mode), func(t *testing.T) {
			if _, errs := ssl.ValidateFunc(mode, "ssl_mode"); len(errs) != 0 {
				t.Fatalf("legacy ssl_mode %q rejected: %v", mode, errs)
			}
			raw := map[string]interface{}{
				"name": "warehouse", "type": "awsredshift",
				"username": "reader", "password": "test-password",
				"database_name": "analytics", "host": "redshift.example.test", "port": "5439",
			}
			if mode != "" {
				raw["ssl_mode"] = mode
			}
			d := schema.TestResourceDataRaw(t, resource.Schema, raw)
			config, err := schemaToResourceIntegrationConfiguration(d, "awsredshift")
			if err != nil {
				t.Fatal(err)
			}
			wire, err := yaml.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			// Decode independently of the provider DTO and reverse mapping: a
			// symmetric mapping bug must not make a round-trip test pass.
			var decoded map[string]interface{}
			if err := yaml.Unmarshal(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			want := map[string]interface{}{
				"version": "", "name": "warehouse", "username": "reader", "password": "test-password",
				"databaseName": "analytics", "hostname": "redshift.example.test", "port": "5439", "sslMode": mode,
			}
			if !reflect.DeepEqual(decoded, want) {
				t.Errorf("wire config = %#v; want %#v", decoded, want)
			}
			if mode == "verify-full" {
				const golden = "version: \"\"\nname: warehouse\nusername: reader\npassword: test-password\ndatabaseName: analytics\nhostname: redshift.example.test\nport: \"5439\"\nsslMode: verify-full\n"
				if string(wire) != golden {
					t.Errorf("wire YAML =\n%s\nwant:\n%s", wire, golden)
				}
			}
		})
	}
	for _, invalid := range []string{"enabled", "VERIFY-FULL"} {
		if _, errs := ssl.ValidateFunc(invalid, "ssl_mode"); len(errs) == 0 {
			t.Errorf("invalid ssl_mode %q accepted", invalid)
		}
	}
}

func TestRedshiftSSLReadContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{"present overwrites", map[string]interface{}{"sslMode": "verify-full"}, "verify-full"},
		{"explicit empty clears", map[string]interface{}{"sslMode": ""}, ""},
		{"missing preserves", map[string]interface{}{"hostname": "remote.example.test"}, "require"},
		{"empty config preserves", map[string]interface{}{}, "require"},
		{"absent config preserves", nil, "require"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Hand-authored server response; never derive it from the forward DTO.
			response := map[string]interface{}{
				"id": "redshift-1", "name": "warehouse", "integrationType": "awsredshift",
				"redactedKeys": []string{"password"},
			}
			if tc.config != nil {
				response["configuration"] = tc.config
			}
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode response: %v", err)
				}
			})
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
				"name": "warehouse", "type": "awsredshift", "ssl_mode": "require",
				"password": "configured-secret", "username": "reader",
			})
			d.SetId("redshift-1")
			if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); len(diags) != 0 {
				t.Fatalf("read diagnostics: %v", diags)
			}
			if got := d.Get("ssl_mode"); got != tc.want {
				t.Errorf("ssl_mode = %q; want %q", got, tc.want)
			}
			if got := d.Get("password"); got != "configured-secret" {
				t.Error("redacted password was not preserved")
			}
			if got := d.Get("username"); got != "reader" {
				t.Errorf("missing username was not preserved: %q", got)
			}
		})
	}
}
