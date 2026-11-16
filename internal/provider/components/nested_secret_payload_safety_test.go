package components

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func nestedPayloadData(t *testing.T, rawPassword cty.Value, plannedPassword string) *schema.ResourceData {
	t.Helper()
	return secretUpdateData(t, map[string]interface{}{
		"name": "fleet", "type": "adaptive_rdp", "tags": []interface{}{"changed"},
		"targets": []interface{}{map[string]interface{}{"id": "target-1", "host": "windows.example", "username": "administrator", "password": plannedPassword}},
	}, map[string]cty.Value{
		"name": cty.StringVal("fleet"), "type": cty.StringVal("adaptive_rdp"),
		"targets": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"id": cty.StringVal("target-1"), "host": cty.StringVal("windows.example"), "username": cty.StringVal("administrator"), "password": rawPassword,
			"port": cty.NullVal(cty.Number), "record": cty.NullVal(cty.Bool),
		})}),
	})
}

func nestedSerializedPayload(t *testing.T, d *schema.ResourceData) []byte {
	t.Helper()
	cfg, err := schemaToResourceIntegrationConfiguration(d, "adaptive_rdp")
	if err != nil {
		t.Fatal("could not construct valid nested payload")
	}
	payload, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal("could not serialize valid nested payload")
	}
	return payload
}

func TestNestedSecretPayloadRejectsSerializedLoss(t *testing.T) {
	for _, held := range []string{"targets", "targets[0].password"} {
		for _, mode := range []string{"missing-targets", "null-targets", "wrong-targets-type", "invalid-embedded-yaml", "empty-targets", "missing-password", "null-password", "changed-password", "trimmed-password", "missing-target", "extra-target", "changed-host", "changed-id", "changed-record"} {
			t.Run(held+"/"+mode, func(t *testing.T) {
				api := &fakeAPI{}
				client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fleet", "integrationType": "adaptive_rdp", "redactedDigests": map[string]string{held: "synthetic-held-fingerprint"}})
				})
				d := nestedPayloadData(t, cty.StringVal(" synthetic-exact-credential "), " synthetic-exact-credential ")
				var outer map[string]interface{}
				if err := yaml.Unmarshal(nestedSerializedPayload(t, d), &outer); err != nil {
					t.Fatal("invalid fixture outer YAML")
				}
				var targets []map[string]interface{}
				if err := yaml.Unmarshal([]byte(outer["targets"].(string)), &targets); err != nil {
					t.Fatal("invalid fixture embedded YAML")
				}
				switch mode {
				case "missing-targets":
					delete(outer, "targets")
				case "null-targets":
					outer["targets"] = nil
				case "wrong-targets-type":
					outer["targets"] = targets
				case "invalid-embedded-yaml":
					outer["targets"] = "["
				default:
					switch mode {
					case "empty-targets", "missing-target":
						targets = nil
					case "extra-target":
						targets = append(targets, targets[0])
					case "missing-password":
						delete(targets[0], "password")
					case "null-password":
						targets[0]["password"] = nil
					case "changed-password":
						targets[0]["password"] = "synthetic-altered-credential"
					case "trimmed-password":
						targets[0]["password"] = "synthetic-exact-credential"
					case "changed-host":
						targets[0]["host"] = "other.example"
					case "changed-id":
						targets[0]["id"] = "other-target"
					case "changed-record":
						targets[0]["record"] = false
					}
					inner, err := yaml.Marshal(targets)
					if err != nil {
						t.Fatal("could not serialize altered fixture")
					}
					outer["targets"] = string(inner)
				}
				payload, err := yaml.Marshal(outer)
				if err != nil {
					t.Fatal("could not serialize altered outer fixture")
				}
				before := d.State().DeepCopy()
				err = validateResourceSecretUpdate(context.Background(), d, client, payload)
				if err == nil {
					t.Error("accepted missing or altered serialized nested configuration")
				}
				if !reflect.DeepEqual(before, d.State()) {
					t.Error("payload verification mutated state")
				}
				if api.sawPath("/update/") {
					t.Error("payload verification invoked mutation endpoint")
				}
				if err != nil {
					for _, secret := range []string{"synthetic-exact-credential", "synthetic-altered-credential", "synthetic-held-fingerprint"} {
						if strings.Contains(err.Error(), secret) {
							t.Error("payload diagnostic exposed a credential or fingerprint")
						}
					}
				}
			})
		}
	}
}

func TestNestedSecretPayloadRejectsUnsafeLeavesAndCollections(t *testing.T) {
	for _, held := range []string{"targets", "targets[0].password"} {
		for _, mode := range []string{"missing-password", "null-password", "unknown-password", "empty-password", "wrong-password-type", "missing-targets", "null-targets", "unknown-targets", "null-target", "unknown-target", "empty-targets", "wrong-targets-type", "changed-id", "changed-host", "changed-username", "changed-port", "changed-name", "changed-domain", "changed-record", "extra-target", "stale-second-password", "reordered-plan", "reordered-payload"} {
			t.Run(held+"/"+mode, func(t *testing.T) {
				api := &fakeAPI{}
				client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fleet", "integrationType": "adaptive_rdp", "redactedKeys": []string{held}})
				})
				d := nestedPayloadData(t, cty.StringVal("synthetic-configured-credential"), "synthetic-configured-credential")
				payload := nestedSerializedPayload(t, d)
				raw := d.GetRawConfig().AsValueMap()
				target := raw["targets"].AsValueSlice()[0].AsValueMap()
				switch mode {
				case "missing-password":
					delete(target, "password")
				case "null-password":
					target["password"] = cty.NullVal(cty.String)
				case "unknown-password":
					target["password"] = cty.UnknownVal(cty.String)
				case "empty-password":
					target["password"] = cty.StringVal("")
				case "wrong-password-type":
					target["password"] = cty.True
				case "changed-id":
					target["id"] = cty.StringVal("other-target")
				case "changed-host":
					target["host"] = cty.StringVal("other.example")
				case "changed-username":
					target["username"] = cty.StringVal("other-user")
				case "changed-port":
					target["port"] = cty.NumberIntVal(3390)
				case "changed-name":
					target["name"] = cty.StringVal("different display")
				case "changed-domain":
					target["domain"] = cty.StringVal("DOMAIN")
				case "changed-record":
					target["record"] = cty.True
				}
				raw["targets"] = cty.ListVal([]cty.Value{cty.ObjectVal(target)})
				switch mode {
				case "missing-targets":
					delete(raw, "targets")
				case "null-targets":
					raw["targets"] = cty.NullVal(raw["targets"].Type())
				case "unknown-targets":
					raw["targets"] = cty.UnknownVal(raw["targets"].Type())
				case "null-target":
					raw["targets"] = cty.ListVal([]cty.Value{cty.NullVal(cty.ObjectVal(target).Type())})
				case "unknown-target":
					raw["targets"] = cty.ListVal([]cty.Value{cty.UnknownVal(cty.ObjectVal(target).Type())})
				case "empty-targets":
					raw["targets"] = cty.ListValEmpty(cty.ObjectVal(target).Type())
				case "wrong-targets-type":
					raw["targets"] = cty.StringVal("not-a-collection")
				case "extra-target":
					raw["targets"] = cty.ListVal([]cty.Value{cty.ObjectVal(target), cty.ObjectVal(target)})
				case "stale-second-password", "reordered-plan", "reordered-payload":
					second := cty.ObjectVal(target).AsValueMap()
					second["id"] = cty.StringVal("target-2")
					second["password"] = cty.StringVal("synthetic-second-credential")
					raw["targets"] = cty.ListVal([]cty.Value{cty.ObjectVal(target), cty.ObjectVal(second)})
					planned := d.Get("targets").([]interface{})
					secondPlan := make(map[string]interface{})
					for key, value := range planned[0].(map[string]interface{}) {
						secondPlan[key] = value
					}
					secondPlan["id"], secondPlan["password"] = "target-2", "synthetic-second-credential"
					planned = append(planned, secondPlan)
					if err := d.Set("targets", planned); err != nil {
						t.Fatal("could not set second fixture target")
					}
					state := d.State()
					state.RawConfig = cty.ObjectVal(raw)
					d = ResourceAdaptiveResource().Data(state)
					payload = nestedSerializedPayload(t, d)
					if mode == "stale-second-password" {
						secondPlan["password"] = "synthetic-stale-second-credential"
					}
					if mode == "reordered-plan" {
						planned[0], planned[1] = planned[1], planned[0]
					}
					if mode == "reordered-payload" {
						var outer map[string]interface{}
						if err := yaml.Unmarshal(payload, &outer); err != nil {
							t.Fatal("invalid outer fixture")
						}
						var inner []map[string]interface{}
						if err := yaml.Unmarshal([]byte(outer["targets"].(string)), &inner); err != nil {
							t.Fatal("invalid inner fixture")
						}
						inner[0], inner[1] = inner[1], inner[0]
						embedded, err := yaml.Marshal(inner)
						if err != nil {
							t.Fatal("could not serialize reordered targets")
						}
						outer["targets"] = string(embedded)
						payload, err = yaml.Marshal(outer)
						if err != nil {
							t.Fatal("could not serialize reordered payload")
						}
					}
					if err := d.Set("targets", planned); err != nil {
						t.Fatal("could not set altered targets")
					}
				}
				state := d.State()
				state.RawConfig = cty.ObjectVal(raw)
				d = ResourceAdaptiveResource().Data(state)
				before := d.State().DeepCopy()
				if err := validateResourceSecretUpdate(context.Background(), d, client, payload); err == nil {
					t.Error("accepted unsafe raw/planned/serialized nested targets")
				}
				if !reflect.DeepEqual(before, d.State()) {
					t.Error("unsafe nested preflight changed state")
				}
				if api.sawPath("/update/") {
					t.Error("unsafe nested preflight invoked mutation endpoint")
				}
			})
		}
	}
}

func TestNestedSecretPayloadAllowsSupportedConfiguredSemantics(t *testing.T) {
	for _, held := range []string{"targets", "targets[0].password"} {
		for _, mode := range []string{"defaults-inherit", "explicit-false", "explicit-true", "whitespace-password", "literal-values", "explicit-optional-empty", "multiple-targets"} {
			t.Run(held+"/"+mode, func(t *testing.T) {
				api := &fakeAPI{}
				client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fleet", "integrationType": "adaptive_rdp", "redactedKeys": []string{held}})
					} else {
						_, _ = w.Write([]byte(`{"success":true}`))
					}
				})
				password := " synthetic-configured-credential \n"
				if mode == "whitespace-password" {
					password = " \t\n"
				}
				d := nestedPayloadData(t, cty.StringVal(password), password)
				raw := d.GetRawConfig().AsValueMap()
				target := raw["targets"].AsValueSlice()[0].AsValueMap()
				planned := d.Get("targets").([]interface{})
				plan := planned[0].(map[string]interface{})
				switch mode {
				case "explicit-false", "explicit-true":
					enabled := mode == "explicit-true"
					target["record"], plan["record"] = cty.BoolVal(enabled), enabled
				case "literal-values":
					for _, key := range []string{"id", "host", "username", "name", "domain"} {
						target[key], plan[key] = cty.StringVal(" literal value "), " literal value "
					}
					target["port"], plan["port"] = cty.NumberIntVal(65535), 65535
				case "explicit-optional-empty":
					target["name"], target["domain"] = cty.StringVal(""), cty.StringVal("")
				}
				rawTargets := []cty.Value{cty.ObjectVal(target)}
				if mode == "multiple-targets" {
					second := cty.ObjectVal(target).AsValueMap()
					second["id"], second["password"] = cty.StringVal("target-2"), cty.StringVal("synthetic-second-credential")
					rawTargets = append(rawTargets, cty.ObjectVal(second))
					secondPlan := make(map[string]interface{})
					for key, value := range plan {
						secondPlan[key] = value
					}
					secondPlan["id"], secondPlan["password"] = "target-2", "synthetic-second-credential"
					planned = append(planned, secondPlan)
				}
				raw["targets"] = cty.ListVal(rawTargets)
				if err := d.Set("targets", planned); err != nil {
					t.Fatal("could not set configured fixture")
				}
				state := d.State()
				state.RawConfig = cty.ObjectVal(raw)
				d = ResourceAdaptiveResource().Data(state)
				payload := nestedSerializedPayload(t, d)
				before := d.State().DeepCopy()
				if err := validateResourceSecretUpdate(context.Background(), d, client, payload); err != nil {
					t.Fatal("rejected supported configured nested semantics")
				}
				if !reflect.DeepEqual(before, d.State()) {
					t.Error("valid nested preflight changed state")
				}
				if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); diags.HasError() {
					t.Fatal("valid configured nested update failed")
				}
				if !api.sawPath("/update/") {
					t.Error("valid configured nested update did not reach fake mutation endpoint")
				}
			})
		}
	}
}

func TestNestedSecretPayloadRejectsIgnoredPlannedCredentialBeforeWrite(t *testing.T) {
	for _, held := range []string{"targets", "targets[0].password"} {
		t.Run(held, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fleet", "integrationType": "adaptive_rdp", "redactedKeys": []string{held}})
				} else {
					_, _ = w.Write([]byte(`{"success":true}`))
				}
			})
			d := nestedPayloadData(t, cty.StringVal("synthetic-configured-credential"), "synthetic-stale-planned-credential")
			before := d.State().DeepCopy()
			diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
			if !diags.HasError() {
				t.Error("accepted a nested planned credential that differs from raw configuration")
			}
			if api.sawPath("/update/") {
				t.Error("unsafe nested credential reached mutation endpoint")
			}
			if !reflect.DeepEqual(before, d.State()) {
				t.Error("rejected nested credential changed state")
			}
			if diags.HasError() {
				message := diags[0].Summary + diags[0].Detail
				if !strings.Contains(message, "targets") || !strings.Contains(message, "ignore_changes") {
					t.Error("diagnostic omitted affected collection or lifecycle conflict")
				}
				for _, secret := range []string{"synthetic-configured-credential", "synthetic-stale-planned-credential"} {
					if strings.Contains(message, secret) {
						t.Error("diagnostic exposed a credential")
					}
				}
			}
		})
	}
}
