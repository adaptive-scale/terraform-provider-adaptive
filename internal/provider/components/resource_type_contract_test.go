package components

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceTypeChangeRejectedDuringPlanning(t *testing.T) {
	r := ResourceAdaptiveResource()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "fixture", "type": "postgres"})
	d.SetId("resource-1")
	state := d.State()
	state.RawConfig = cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("fixture"), "type": cty.StringVal("mysql")})
	_, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "fixture", "type": "mysql"}), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot change resource integration type") {
		t.Fatal("unsupported resource integration type transition was accepted during planning")
	}
}

func TestResourceRenameRemainsMutable(t *testing.T) {
	r := ResourceAdaptiveResource()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "before", "type": "postgres"})
	d.SetId("resource-1")
	state := d.State()
	state.RawConfig = cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("after"), "type": cty.StringVal("postgres")})
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "after", "type": "postgres"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.RequiresNew() {
		t.Fatal("supported resource rename must remain an in-place change")
	}
}
