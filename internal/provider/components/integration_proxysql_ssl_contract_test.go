package components

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"gopkg.in/yaml.v2"
)

// Source contracts: proxysql/config.go form suggestions and GenerateConfig;
// mysql/config.go GenerateMySQLConnectionCommand forwards SSLMode literally.
// Canonical modes also satisfy CLI form validation. The historical require
// prefix remains runtime-compatible but is not accepted by the CLI form route.
func TestIntegrationProxySQLSSLModeContract(t *testing.T) {
	for _, mode := range []string{"verify_identity", "verify_ca", "required", "preferred", "disabled", "require", ""} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]interface{}{"name": "fixture", "type": "proxysql", "ssl_mode": mode}
			if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err != nil {
				t.Fatalf("supported ProxySQL mode rejected during planning: %v", err)
			}
			for _, update := range []bool{false, true} {
				t.Run(map[bool]string{false: "create", true: "update"}[update], func(t *testing.T) {
					api := &fakeAPI{}
					client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
						if r.Method == http.MethodGet {
							_, _ = w.Write([]byte(`{"id":"fixture-id","name":"fixture","integrationType":"proxysql","configuration":{}}`))
							return
						}
						_, _ = w.Write([]byte(`{"id":"fixture-id","success":true}`))
					})
					d := parityData(t, "proxysql", map[string]interface{}{"ssl_mode": mode})
					if update {
						if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); diags.HasError() {
							t.Fatalf("update failed: %v", diags)
						}
					} else {
						d.SetId("")
						if diags := ResourceAdaptiveResourceCreate(context.Background(), d, client); diags.HasError() {
							t.Fatalf("create failed: %v", diags)
						}
					}
					wantPath := "/api/v1/terraform/resource/create"
					if update {
						wantPath = "/api/v1/terraform/resource/update/fixture-id"
					}
					writes := 0
					for i, path := range api.paths {
						if path != wantPath {
							if !strings.Contains(path, "/read/") {
								t.Errorf("unexpected route %s", path)
							}
							continue
						}
						writes++
						if api.body[i]["integrationType"] != "proxysql" {
							t.Error("wrong wire integration type")
						}
						var wire map[string]interface{}
						if err := yaml.UnmarshalStrict([]byte(api.body[i]["config"].(string)), &wire); err != nil {
							t.Fatal(err)
						}
						if wire["sslMode"] != mode {
							t.Errorf("wire SSL mode = %v; want literal %q", wire["sslMode"], mode)
						}
					}
					if writes != 1 {
						t.Errorf("writes = %d; want 1", writes)
					}
					if err := integrations.ApplyIntegrationConfig(d, "proxysql", map[string]interface{}{"sslMode": mode}); err != nil {
						t.Fatal(err)
					}
					if got := parityWire(t, d, "proxysql")["sslMode"]; got != mode {
						t.Errorf("refresh mode = %v; want literal %q", got, mode)
					}
				})
			}
		})
	}
	t.Run("omitted-default", func(t *testing.T) {
		if got := parityWire(t, parityData(t, "proxysql", map[string]interface{}{}), "proxysql")["sslMode"]; got != "" {
			t.Errorf("omitted mode = %v; want empty (omit the runtime SSL-mode flag)", got)
		}
	})
}

func TestIntegrationProxySQLRejectsNonRuntimeSSLModes(t *testing.T) {
	for _, mode := range []string{"verify-full", "verify-ca", "allow", "prefer", "disable"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]interface{}{"name": "fixture", "type": "proxysql", "ssl_mode": mode}
			if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err == nil || !strings.Contains(err.Error(), "ProxySQL ssl_mode") {
				t.Errorf("unsupported mode accepted during planning or wrong diagnostic: %v", err)
			}
			for _, update := range []bool{false, true} {
				api := &fakeAPI{}
				client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`{"id":"fixture-id","success":true}`))
				})
				d := parityData(t, "proxysql", map[string]interface{}{"ssl_mode": mode})
				if update {
					if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() || !strings.Contains(diags[0].Summary, "ProxySQL ssl_mode") {
						t.Errorf("invalid mode accepted on update: %v", diags)
					}
				} else {
					d.SetId("")
					if diags := ResourceAdaptiveResourceCreate(context.Background(), d, client); !diags.HasError() || !strings.Contains(diags[0].Summary, "ProxySQL ssl_mode") {
						t.Errorf("invalid mode accepted on create: %v", diags)
					}
				}
				if len(api.paths) != 0 {
					t.Errorf("invalid mode reached HTTP: %v", api.paths)
				}
			}
		})
	}
}

func TestIntegrationProxySQLNativeSSLModesRemainTypeScoped(t *testing.T) {
	for _, typ := range []string{"postgres", "awsredshift", "sql_server", "services"} {
		for _, mode := range []string{"verify_identity", "verify_ca", "required", "preferred", "disabled"} {
			if err := validateResourceContractInputs(typ, func(attr string) interface{} {
				if attr == "ssl_mode" {
					return mode
				}
				return nil
			}); err == nil {
				t.Errorf("%s accepted MySQL-family mode %q", typ, mode)
			}
		}
	}
}
