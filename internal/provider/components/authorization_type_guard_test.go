package components

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAuthorizationResourceTypeChangeRejected(t *testing.T) {
	r := ResourceAdaptiveAuthorization()
	values := map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": "SELECT"}
	original := schema.TestResourceDataRaw(t, r.Schema, values)
	original.SetId("existing")
	values["resource_type"] = "postgres"
	if _, err := r.Diff(context.Background(), original.State(), terraform.NewResourceConfigRaw(values), nil); err == nil || !strings.Contains(err.Error(), "resource_type cannot be updated") {
		t.Fatalf("unsafe plan accepted: %v", err)
	}
	d, err := schema.InternalMap(r.Schema).Data(original.State(), &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{"resource_type": {Old: "mysql", New: "postgres"}}})
	if err != nil {
		t.Fatal(err)
	}
	// A nil client makes any attempt to reach transport fail the test.
	diags := r.UpdateContext(context.Background(), d, nil)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "resource_type cannot be updated") || d.Id() != "existing" {
		t.Fatal("missing direct callback guard", diags)
	}
}

func TestAuthorizationResourceTypeCreateAndUnknownPlan(t *testing.T) {
	r := ResourceAdaptiveAuthorization()
	for resourceType := range validAuthorizedResourceTypes {
		values := map[string]interface{}{"name": "role", "resource_type": resourceType, "permissions": "SELECT"}
		if _, err := r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err != nil {
			t.Fatal(resourceType, err)
		}
	}
	original := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": "SELECT"})
	original.SetId("existing")
	for _, v := range []string{"mysql", "${unknown}"} {
		if v == "${unknown}" {
			v = "74D93920-ED26-11E3-AC10-0800200C9A66"
		}
		if _, err := r.Diff(context.Background(), original.State(), terraform.NewResourceConfigRaw(map[string]interface{}{"name": "role", "resource_type": v, "permissions": "SELECT"}), nil); err != nil {
			t.Fatal("unchanged/unknown type must defer", err)
		}
	}
}
