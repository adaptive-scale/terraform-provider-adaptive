package components

import (
	"context"
	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"gopkg.in/yaml.v2"
	"reflect"
	"testing"
)

func TestIntegrationParitySDKOwnership(t *testing.T) {
	fixtures := append([]parityFieldFixture{}, tlsIdentityFixtures...)
	for typ, text := range iamBackendFixtures {
		names := map[string]string{}
		var wire map[string]interface{}
		_ = yaml.Unmarshal([]byte(text), &wire)
		for k := range wire {
			if a, ok := integrations.AttrForConfigKey(typ, k); ok {
				names[k] = a
			}
		}
		fixtures = append(fixtures, parityFieldFixture{typ, text, names})
	}
	fixtures = append(fixtures, parityFieldFixture{"aruba_sw", "automationMode: auto\nlogin_url: /login\nport: '2222'\nwebui_port: '8443'\n", map[string]string{"automationMode": "automation_mode", "login_url": "login_url", "port": "port", "webui_port": "webui_port"}})
	for _, tc := range fixtures {
		for _, clear := range []bool{false, true} {
			suffix := "/omit"
			if clear {
				suffix = "/clear"
			}
			t.Run(tc.typ+suffix, func(t *testing.T) {
				r := ResourceAdaptiveResource()
				d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"type": tc.typ, "name": "fixture"})
				d.SetId("r-1")
				var wire map[string]interface{}
				if err := yaml.UnmarshalStrict([]byte(tc.yaml), &wire); err != nil {
					t.Fatal(err)
				}
				// Secret values cannot be imported; held-secret handling is tested separately.
				for k := range wire {
					a := tc.names[k]
					if a == "" || r.Schema[a].Sensitive {
						delete(wire, k)
					}
				}
				if err := integrations.ApplyIntegrationConfig(d, tc.typ, wire); err != nil {
					t.Fatal(err)
				}
				values := map[string]interface{}{"name": "fixture", "type": tc.typ, "tags": []interface{}{"changed"}}
				raw := map[string]cty.Value{"name": cty.StringVal("fixture"), "type": cty.StringVal(tc.typ)}
				if clear {
					for k, v := range wire {
						a := tc.names[k]
						if _, ok := v.(bool); ok {
							values[a] = false
							raw[a] = cty.BoolVal(false)
						} else {
							values[a] = ""
							raw[a] = cty.StringVal("")
						}
					}
				}
				state := d.State()
				state.RawConfig = cty.ObjectVal(raw)
				diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(values), nil)
				if err != nil {
					t.Fatal(err)
				}
				planned, err := schema.InternalMap(r.Schema).Data(state, diff)
				if err != nil {
					t.Fatal(err)
				}
				for k, v := range wire {
					a := tc.names[k]
					want := v
					if clear {
						if _, ok := v.(bool); ok {
							want = false
						} else {
							want = ""
						}
					}
					if !reflect.DeepEqual(planned.Get(a), want) {
						t.Errorf("%s omission/clear lost observed ownership", a)
					}
				}
			})
		}
	}
}
func TestIntegrationParityFailsClosedWithoutOwnership(t *testing.T) {
	for _, typ := range []string{"mysql", "postgres", "kafka", "awselasticcache", "mongodb", "syslog", "google", "rdp_windows", "services", "awsdocumentdb", "sql_server", "azuresqlserver", "aruba_sw"} {
		if err := validateCloudConfigOwnership(typ, true, cty.NullVal(cty.DynamicPseudoType)); err == nil {
			t.Errorf("%s accepts full replacement without ownership", typ)
		}
	}
}
func TestIntegrationParityUnusedAndDeviceAliasInputs(t *testing.T) {
	for _, tc := range []struct{ typ, attr string }{{"mysql", "version"}, {"syslog", "app_key"}, {"cisco_ngfw", "login_url"}, {"fortinet_ngfw", "login_url"}, {"hpe_switch", "login_url"}} {
		if err := validateResourceContractInputs(tc.typ, func(a string) interface{} {
			if a == tc.attr {
				return "configured"
			}
			return nil
		}); err == nil {
			t.Errorf("%s/%s silently ignored", tc.typ, tc.attr)
		}
	}
}

func TestIntegrationParityUnsupportedInputs(t *testing.T) {
	for _, tc := range []struct {
		typ, attr string
		v         interface{}
	}{
		{"mysql", "use_msk_iam", true}, {"kafka", "use_rds_iam", true}, {"postgres", "use_iam_auth", true}, {"mysql", "admin_email", "a"}, {"google", "allow_file_transfer", true},
		{"aruba_sw", "prestart", "unsupported"}, {"mysql", "automation_mode", "auto"}, {"mysql", "app_id", "unused"}, {"snowflake", "database_account", "unused"}, {"snowflake", "database_username", "unused"}, {"snowflake", "database_password", "unused"},
	} {
		t.Run(tc.typ+"/"+tc.attr, func(t *testing.T) {
			if err := validateResourceContractInputs(tc.typ, func(a string) interface{} {
				if a == tc.attr {
					return tc.v
				}
				return nil
			}); err == nil {
				t.Error("unsupported input was silently accepted")
			}
		})
	}
}
