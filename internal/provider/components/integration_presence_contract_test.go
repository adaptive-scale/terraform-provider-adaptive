package components

import (
	"context"
	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"testing"
)

func TestIntegrationIAMSDKObservedSwitchPresence(t *testing.T) {
	for _, typ := range []string{"mysql", "postgres", "kafka", "awselasticcache"} {
		for _, mode := range []string{"absent", "null", "false", "true"} {
			t.Run(typ+"/"+mode, func(t *testing.T) {
				attr, key := "use_rds_iam", "useRdsIam"
				if typ == "kafka" {
					attr, key = "use_msk_iam", "useMskIam"
				}
				if typ == "awselasticcache" {
					attr, key = "use_iam_auth", "useIamAuth"
				}
				r := ResourceAdaptiveResource()
				d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "fixture", "type": typ})
				d.SetId("r-1")
				cfg := map[string]interface{}{}
				if typ == "mysql" || typ == "postgres" {
					cfg["authMode"] = "aws_rds_iam"
				}
				if mode == "null" {
					cfg[key] = nil
				}
				if mode == "false" || mode == "true" {
					cfg[key] = mode == "true"
				}
				if err := integrations.ApplyIntegrationConfig(d, typ, cfg); err != nil {
					t.Fatal(err)
				}
				state := d.State()
				state.RawConfig = cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("fixture"), "type": cty.StringVal(typ)})
				diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "fixture", "type": typ, "tags": []interface{}{"changed"}}), nil)
				if err != nil {
					t.Fatal(err)
				}
				planned, err := schema.InternalMap(r.Schema).Data(state, diff)
				if err != nil {
					t.Fatal(err)
				}
				wire := parityWire(t, planned, typ)
				v, present := wire[key]
				want := mode == "false" || mode == "true"
				if present != want || (present && v != (mode == "true")) {
					t.Errorf("observed %s presence changed during unrelated update", attr)
				}
				if _, present := wire["useIrsa"]; present {
					t.Error("omitted IRSA flattened into false during SDK update")
				}
			})
		}
	}
}

func TestIntegrationParityUnsupportedExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		typ, attr string
		v         interface{}
	}{
		{"mysql", "use_msk_iam", false}, {"kafka", "use_rds_iam", false}, {"postgres", "use_iam_auth", false}, {"google", "allow_file_transfer", false}, {"mysql", "admin_email", ""}, {"aruba_sw", "prestart", ""},
	} {
		t.Run(tc.typ+"/"+tc.attr, func(t *testing.T) {
			r := ResourceAdaptiveResource()
			d := parityData(t, tc.typ, map[string]interface{}{tc.attr: tc.v})
			state := d.State()
			state.RawConfig = d.GetRawConfig()
			if _, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "fixture", "type": tc.typ, tc.attr: tc.v}), nil); err == nil {
				t.Error("explicit unsupported false/empty was silently accepted")
			}
		})
	}
}
func TestIntegrationParityBackendEnums(t *testing.T) {
	for _, tc := range []struct{ typ, attr string }{{"mysql", "auth_mode"}, {"postgres", "auth_mode"}, {"awselasticcache", "cache_type"}} {
		if err := validateResourceContractInputs(tc.typ, func(a string) interface{} {
			if a == tc.attr {
				return "unsupported-enum"
			}
			return nil
		}); err == nil {
			t.Errorf("invalid %s accepted", tc.attr)
		}
	}
	for _, mode := range []string{"false", "skip-verify"} {
		if _, errs := ResourceAdaptiveResource().Schema["ssl_mode"].ValidateFunc(mode, "ssl_mode"); len(errs) > 0 {
			t.Error("shared schema rejects backend SQL Server SSL mode")
		}
	}
}
