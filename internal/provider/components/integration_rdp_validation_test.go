package components

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"gopkg.in/yaml.v2"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func rdpValidationValues(field string, value interface{}) map[string]interface{} {
	target := map[string]interface{}{"id": "target-1", "host": "windows.example", "username": "administrator", "password": "literal-secret"}
	target[field] = value
	return map[string]interface{}{"name": "fleet", "type": "adaptive_rdp", "targets": []interface{}{target}}
}

// Exercise the registered schema, real SDK diff, constructor and mutation
// callbacks independently: Required alone permits explicitly blank strings.
func TestAdaptiveRDPTargetRejectsBlankIdentity(t *testing.T) {
	for _, field := range []string{"id", "host", "username"} {
		for _, value := range []string{"", " \t\n", "\u2003"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				testRDPInvalidBoundaries(t, rdpValidationValues(field, value), field)
			})
		}
	}
}

func TestAdaptiveRDPTargetRejectsInvalidTCPPort(t *testing.T) {
	for _, port := range []int{-1, 0, 65536, 2147483647} {
		t.Run(fmt.Sprint(port), func(t *testing.T) {
			testRDPInvalidBoundaries(t, rdpValidationValues("port", port), "port")
		})
	}
}

// Regression checks use the actual registered target schema, not a copy.
func TestAdaptiveRDPTargetContractPreserved(t *testing.T) {
	r := ResourceAdaptiveResource()
	leaves := r.Schema["targets"].Elem.(*schema.Resource).Schema
	if len(leaves) != 8 {
		t.Fatalf("RDP target leaf count changed: %d", len(leaves))
	}
	for _, field := range []string{"id", "name", "host", "port", "username", "password", "domain", "record"} {
		if leaves[field] == nil {
			t.Fatalf("missing RDP target leaf %s", field)
		}
	}
	if !leaves["password"].Sensitive || !leaves["password"].Required || leaves["port"].Default != 3389 {
		t.Fatal("RDP password or default-port schema contract changed")
	}
	for _, port := range []int{1, 3389, 65535} {
		t.Run(fmt.Sprint(port), func(t *testing.T) {
			values := rdpValidationValues("port", port)
			config := terraform.NewResourceConfigRaw(values)
			if diags := r.Validate(config); diags.HasError() {
				t.Fatalf("valid TCP port rejected by schema: %v", diags)
			}
			if _, err := r.Diff(context.Background(), nil, config, nil); err != nil {
				t.Fatalf("valid TCP port rejected during planning: %v", err)
			}
			cfg, err := integrations.SchemaToAdaptiveRDPIntegrationConfiguration(schema.TestResourceDataRaw(t, r.Schema, values))
			if err != nil {
				t.Fatal(err)
			}
			var targets []integrations.AdaptiveRDPTargetConfig
			if err := yaml.Unmarshal([]byte(cfg.Targets), &targets); err != nil {
				t.Fatal(err)
			}
			if len(targets) != 1 || targets[0].Port != port {
				t.Fatal("valid TCP port changed in payload")
			}
		})
	}
	for _, record := range []cty.Value{cty.NullVal(cty.Bool), cty.False, cty.True} {
		t.Run(record.GoString(), func(t *testing.T) {
			values := rdpValidationValues("id", " target-1 ")
			target := values["targets"].([]interface{})[0].(map[string]interface{})
			target["name"], target["host"], target["username"] = " display ", " windows.example ", " user "
			target["password"], target["domain"] = " \t\n", " DOMAIN "
			// Port omitted: the schema must still supply 3389.
			rawTarget := map[string]cty.Value{"port": cty.NullVal(cty.Number), "record": record}
			for key, v := range target {
				rawTarget[key] = cty.StringVal(v.(string))
			}
			d := secretUpdateData(t, values, map[string]cty.Value{
				"name": cty.StringVal("fleet"), "type": cty.StringVal("adaptive_rdp"),
				"targets": cty.ListVal([]cty.Value{cty.ObjectVal(rawTarget)}),
			})
			cfg, err := integrations.SchemaToAdaptiveRDPIntegrationConfiguration(d)
			if err != nil {
				t.Fatal(err)
			}
			var targets []integrations.AdaptiveRDPTargetConfig
			if err := yaml.Unmarshal([]byte(cfg.Targets), &targets); err != nil {
				t.Fatal(err)
			}
			got := targets[0]
			if got.ID != target["id"] || got.Name != target["name"] || got.Host != target["host"] || got.Username != target["username"] || got.Password != target["password"] || got.Domain != target["domain"] || got.Port != 3389 {
				t.Fatal("literal target values or default port changed")
			}
			if record.IsNull() {
				if got.Record != nil {
					t.Fatal("absent record no longer inherits global setting")
				}
			} else if got.Record == nil || *got.Record != record.True() {
				t.Fatal("explicit record override changed")
			}
		})
	}
}

func TestAdaptiveRDPTargetUnknownLeavesDeferToApply(t *testing.T) {
	const unknown = "74D93920-ED26-11E3-AC10-0800200C9A66" // SDK hcl2shim unknown sentinel.
	for _, field := range []string{"id", "host", "username", "port"} {
		t.Run(field, func(t *testing.T) {
			if _, err := ResourceAdaptiveResource().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(rdpValidationValues(field, unknown)), nil); err != nil {
				t.Fatalf("unknown target leaf rejected during planning: %v", err)
			}
		})
	}
}

func TestAdaptiveRDPTargetExistingGuards(t *testing.T) {
	for _, mode := range []string{"empty-password", "duplicate-id", "non-rdp"} {
		t.Run(mode, func(t *testing.T) {
			values := rdpValidationValues("port", 3389)
			typ, want := "adaptive_rdp", "password"
			targets := values["targets"].([]interface{})
			switch mode {
			case "empty-password":
				targets[0].(map[string]interface{})["password"] = ""
			case "duplicate-id":
				values["targets"] = append(targets, targets[0])
				want = "duplicate"
			case "non-rdp":
				typ, want = "rdp_windows", "only supported"
				values["type"] = typ
			}
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, values)
			if _, err := schemaToResourceIntegrationConfiguration(d, typ); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("existing target guard lost: %v", err)
			}
		})
	}
	for _, mode := range []string{"no-targets", "no-password", "no-raw-ownership"} {
		t.Run("withheld/"+mode, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, req *http.Request) {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"fleet","integrationType":"adaptive_rdp","redactedKeys":["targets"],"redactedDigests":{"targets":"held"}}`))
			})
			values := rdpValidationValues("port", 3389)
			if mode == "no-targets" {
				delete(values, "targets")
			}
			if mode == "no-password" {
				delete(values["targets"].([]interface{})[0].(map[string]interface{}), "password")
			}
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, values)
			d.SetId("r-1")
			if diags := ResourceAdaptiveResource().UpdateContext(context.Background(), d, client); !diags.HasError() {
				t.Fatal("update accepted withheld credentials without safe resupply/ownership")
			}
			for _, path := range api.paths {
				if !strings.Contains(path, "/read/") {
					t.Fatalf("withheld credential guard allowed write: %s", path)
				}
			}
		})
	}
}

func testRDPInvalidBoundaries(t *testing.T, values map[string]interface{}, field string) {
	t.Helper()
	r := ResourceAdaptiveResource()
	config := terraform.NewResourceConfigRaw(values)
	if diags := r.Validate(config); !diags.HasError() {
		t.Errorf("schema accepted invalid target %s", field)
	}
	if _, err := integrations.SchemaToAdaptiveRDPIntegrationConfiguration(schema.TestResourceDataRaw(t, r.Schema, values)); err == nil || !strings.Contains(err.Error(), field) {
		t.Errorf("constructor accepted invalid target %s or wrong diagnostic: %v", field, err)
	}
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "update"}[existing], func(t *testing.T) {
			var state *terraform.InstanceState
			if existing {
				prior := schema.TestResourceDataRaw(t, r.Schema, rdpValidationValues("port", 3389))
				prior.SetId("r-1")
				state = prior.State()
			}
			if _, err := r.Diff(context.Background(), state, config, nil); err == nil || !strings.Contains(err.Error(), field) {
				t.Errorf("plan accepted invalid target %s or wrong diagnostic: %v", field, err)
			}
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, req *http.Request) {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"fleet","integrationType":"adaptive_rdp"}`))
			})
			d := schema.TestResourceDataRaw(t, r.Schema, values)
			callback := r.CreateContext
			if existing {
				d.SetId("r-1")
				callback = schema.CreateContextFunc(r.UpdateContext)
			}
			if diags := callback(context.Background(), d, client); !diags.HasError() || !strings.Contains(diags[0].Summary, field) {
				t.Errorf("callback accepted invalid target %s or wrong diagnostic: %v", field, diags)
			}
			if len(api.paths) != 0 {
				t.Errorf("invalid target reached HTTP: %v", api.paths)
			}
		})
	}
}
