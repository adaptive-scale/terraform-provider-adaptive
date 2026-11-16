package components

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"gopkg.in/yaml.v2"
)

func TestClickHouseCanonicalWireContract(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[useTLS], func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
				"name": "analytics", "type": "clickhouse", "database_name": "reports",
				"username": "reader", "password": "synthetic-password", "host": "clickhouse.example.test",
				"port": "9440", "use_tls": useTLS,
			})
			obj, err := schemaToResourceIntegrationConfiguration(d, "clickhouse")
			if err != nil {
				t.Fatal("could not build ClickHouse configuration")
			}
			data, err := yaml.Marshal(obj)
			if err != nil {
				t.Fatal("could not encode ClickHouse configuration")
			}
			var wire map[string]interface{}
			if err := yaml.Unmarshal(data, &wire); err != nil {
				t.Fatal("could not decode wire contract")
			}
			if wire["database"] != "reports" || wire["use-tls"] != useTLS {
				t.Error("ClickHouse canonical database/TLS contract not emitted")
			}
			for _, obsolete := range []string{"databaseName", "sslMode"} {
				if _, ok := wire[obsolete]; ok {
					t.Error("obsolete ClickHouse configuration key emitted")
				}
			}
			// This independent shape mirrors the backend rather than this provider's DTO.
			var backend struct {
				Database string `yaml:"database"`
				TLS      bool   `yaml:"use-tls"`
				Host     string `yaml:"hostname"`
			}
			if err := yaml.UnmarshalStrict(data, &struct {
				Name     string `yaml:"name"`
				Username string `yaml:"username"`
				Password string `yaml:"password"`
				Database string `yaml:"database"`
				Host     string `yaml:"hostname"`
				Port     string `yaml:"port"`
				TLS      bool   `yaml:"use-tls"`
			}{}); err != nil {
				t.Fatal("wire payload violates backend key/type shape")
			}
			if err := yaml.Unmarshal(data, &backend); err != nil || backend.Database != "reports" || backend.TLS != useTLS || backend.Host != "clickhouse.example.test" {
				t.Error("independent backend-compatible decode lost configuration")
			}
		})
	}
}

func TestClickHouseCanonicalRefreshContract(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "analytics", "type": "clickhouse", "database_name": "prior", "use_tls": true,
		"password": "synthetic-password",
	})
	if err := integrations.ApplyIntegrationConfig(d, "clickhouse", map[string]interface{}{
		"database": "current", "use-tls": false,
	}); err != nil {
		t.Fatal("could not refresh ClickHouse")
	}
	if d.Get("database_name") != "current" || d.Get("use_tls") != false {
		t.Error("canonical refresh ignored database/TLS removal")
	}
	if d.Get("password") != "synthetic-password" {
		t.Error("withheld credential was overwritten")
	}
	if err := integrations.ApplyIntegrationConfig(d, "clickhouse", map[string]interface{}{"database": ""}); err != nil {
		t.Fatal("could not refresh explicit clear")
	}
	if d.Get("database_name") != "" || d.Get("use_tls") != false {
		t.Error("clear or missing-key semantics changed")
	}
}

func TestClickHouseTLSIsOptionalComputed(t *testing.T) {
	s, ok := ResourceAdaptiveResource().Schema["use_tls"]
	if !ok || !s.Optional || !s.Computed || s.Type != schema.TypeBool {
		t.Error("TLS must be explicitly configurable while retaining an observed value when omitted")
	}
}

func TestClickHouseRejectsUnsupportedLegacySSLMode(t *testing.T) {
	for _, mode := range []string{"prefer", "allow", "require", "verify-ca", "verify-full", "disable"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]interface{}{"name": "analytics", "type": "clickhouse", "ssl_mode": mode}
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, values)
			if _, err := schemaToResourceIntegrationConfiguration(d, "clickhouse"); err == nil || !strings.Contains(err.Error(), "use_tls") {
				t.Error("unsupported ssl_mode was not rejected with migration guidance")
			}
			if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err == nil {
				t.Error("unsupported ssl_mode was not rejected during planning")
			}
		})
	}
}

func TestArubaManagedHostnameAliasSurvivesRefreshWithoutRawConfig(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "device", "type": "aruba_instant_on", "hostname": "prior.invalid",
	})
	d.SetId("r-1")
	if err := integrations.ApplyIntegrationConfig(d, "aruba_instant_on", map[string]interface{}{"hostname": "current.invalid"}); err != nil {
		t.Fatal(err)
	}
	if d.Get("host") != "" || d.Get("hostname") != "current.invalid" {
		t.Error("managed hostname-only state moved to the legacy host alias during refresh")
	}
}

func TestResourceUnsupportedArubaInputsFailBeforeHTTP(t *testing.T) {
	for _, values := range []map[string]interface{}{
		{"name": "device", "type": "aruba_instant_on", "api_token": "synthetic-token"},
		{"name": "device", "type": "aruba_instant_on", "host": "one.invalid", "hostname": "two.invalid"},
	} {
		api := &fakeAPI{}
		client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"r-1","name":"device","integrationType":"aruba_instant_on"}`))
		})
		d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, values)
		if diags := ResourceAdaptiveResourceCreate(context.Background(), d, client); !diags.HasError() {
			t.Error("invalid Aruba input accepted")
		}
		if len(api.paths) != 0 {
			t.Error("invalid Aruba input reached HTTP")
		}
		if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err == nil {
			t.Error("invalid Aruba input accepted during plan")
		}
	}
}

func TestClickHouseRejectsUnsupportedSSLBeforeHTTP(t *testing.T) {
	for _, callback := range []struct {
		name   string
		update bool
	}{{"create", false}, {"update", true}} {
		t.Run(callback.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"analytics","integrationType":"clickhouse"}`))
			})
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
				"name": "analytics", "type": "clickhouse", "ssl_mode": "verify-full",
			})
			if callback.update {
				d.SetId("r-1")
				if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
					t.Error("unsafe update accepted")
				}
			} else if diags := ResourceAdaptiveResourceCreate(context.Background(), d, client); !diags.HasError() {
				t.Error("unsafe create accepted")
			}
			if len(api.paths) != 0 {
				t.Error("unsupported ClickHouse SSL intent reached HTTP")
			}
		})
	}
}
