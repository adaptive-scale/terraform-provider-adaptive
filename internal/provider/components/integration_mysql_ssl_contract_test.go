package components

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestIntegrationMySQLSSLModeContract(t *testing.T) {
	// Empty clears --ssl-mode; require is the historical, CLI-accepted prefix
	// of required. All tokens must round-trip literally, without normalization.
	for _, mode := range []string{"verify_identity", "verify_ca", "required", "preferred", "disabled", "require", ""} {
		t.Run("configured/"+mode, func(t *testing.T) {
			d := parityData(t, "mysql", map[string]interface{}{"ssl_mode": mode})
			if got := parityWire(t, d, "mysql")["sslMode"]; got != mode {
				t.Error("configured MySQL SSL mode ignored")
			}
			if _, errs := ResourceAdaptiveResource().Schema["ssl_mode"].ValidateFunc(mode, "ssl_mode"); len(errs) != 0 {
				t.Error("backend-supported MySQL SSL mode rejected")
			}
			values := map[string]interface{}{"name": "fixture", "type": "mysql", "ssl_mode": mode}
			if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err != nil {
				t.Fatalf("valid MySQL mode rejected during planning: %v", err)
			}
			if err := integrations.ApplyIntegrationConfig(d, "mysql", map[string]interface{}{"sslMode": mode}); err != nil {
				t.Fatal(err)
			}
			if got := parityWire(t, d, "mysql")["sslMode"]; got != mode {
				t.Error("observed MySQL mode normalized or lost")
			}
		})
	}
	t.Run("legacy-default", func(t *testing.T) {
		if got := parityWire(t, parityData(t, "mysql", map[string]interface{}{}), "mysql")["sslMode"]; got != "require" {
			t.Error("legacy MySQL default changed")
		}
	})
	t.Run("reverse-empty", func(t *testing.T) {
		d := parityData(t, "mysql", map[string]interface{}{})
		if err := integrations.ApplyIntegrationConfig(d, "mysql", map[string]interface{}{"sslMode": ""}); err != nil {
			t.Fatal(err)
		}
		if got := parityWire(t, d, "mysql")["sslMode"]; got != "" {
			t.Error("observed empty mode replaced by default")
		}
	})
	testParityFields(t, parityFieldFixture{"mysql", "sslMode: verify_identity\n", map[string]string{"sslMode": "ssl_mode"}})
}

func TestIntegrationMySQLRejectsNonRuntimeSSLModes(t *testing.T) {
	// prefer/disable are accepted abbreviations by this mysql CLI, but are
	// PostgreSQL spellings, not part of our explicit MySQL alias contract.
	for _, mode := range []string{"verify-full", "verify-ca", "allow", "prefer", "disable"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]interface{}{"name": "fixture", "type": "mysql", "ssl_mode": mode}
			r := ResourceAdaptiveResource()
			if _, err := r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err == nil || !strings.Contains(err.Error(), "MySQL ssl_mode") {
				t.Errorf("invalid explicit MySQL mode accepted during planning or wrong diagnostic: %v", err)
			}
			for _, update := range []bool{false, true} {
				t.Run(map[bool]string{false: "create", true: "update"}[update], func(t *testing.T) {
					api := &fakeAPI{}
					client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
						_, _ = w.Write([]byte(`{"id":"r-1","name":"fixture","integrationType":"mysql"}`))
					})
					d := parityData(t, "mysql", map[string]interface{}{"ssl_mode": mode})
					if !update {
						d.SetId("")
					}
					if update {
						d.SetId("r-1")
						if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() || !strings.Contains(diags[0].Summary, "MySQL ssl_mode") {
							t.Errorf("invalid explicit MySQL mode accepted on update or wrong diagnostic: %v", diags)
						}
					} else if diags := ResourceAdaptiveResourceCreate(context.Background(), d, client); !diags.HasError() || !strings.Contains(diags[0].Summary, "MySQL ssl_mode") {
						t.Errorf("invalid explicit MySQL mode accepted on create or wrong diagnostic: %v", diags)
					}
					if len(api.paths) != 0 {
						t.Errorf("invalid explicit MySQL mode reached HTTP: %v", api.paths)
					}
				})
			}
		})
	}
}
