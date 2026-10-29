package components

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestJITGroupExplicitClearIsARealPlanChange(t *testing.T) {
	resource := ResourceAdaptiveSession()
	prior := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{"name": "ep", "resource": "db", "jit_approver_groups": []interface{}{"On call"}})
	prior.SetId("ep")
	diff, err := resource.Diff(context.Background(), prior.State(), terraform.NewResourceConfigRaw(map[string]interface{}{"name": "ep", "resource": "db", "jit_approver_groups": []interface{}{}}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Attributes["jit_approver_groups.#"] == nil || diff.Attributes["jit_approver_groups.#"].New != "0" || diff.Attributes["jit_approver_groups.#"].NewComputed {
		t.Fatalf("explicit [] did not plan clearing groups: %#v", diff)
	}
}

func TestJITGroupRawConfigurationPreservesPresence(t *testing.T) {
	resource := ResourceAdaptiveSession()
	for _, tc := range []struct {
		name    string
		value   cty.Value
		present bool
		count   int
	}{
		{"omitted", cty.NullVal(cty.Set(cty.String)), false, 0},
		{"clear", cty.SetValEmpty(cty.String), true, 0},
		{"selected", cty.SetVal([]cty.Value{cty.StringVal("On call")}), true, 1},
		{"unknown", cty.UnknownVal(cty.Set(cty.String)), false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &terraform.InstanceState{ID: "ep", Attributes: map[string]string{}, RawConfig: cty.ObjectVal(map[string]cty.Value{"jit_approver_groups": tc.value})}
			groups := configuredJITApproverGroups(resource.Data(state))
			if (groups != nil) != tc.present {
				t.Fatalf("group presence changed: %v", groups)
			}
			if groups != nil && len(*groups) != tc.count {
				t.Fatalf("lost group values: %v", *groups)
			}
		})
	}
}
