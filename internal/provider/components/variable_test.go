package components

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestAdaptiveVariableSchemaProtectsIdentityAndValue(t *testing.T) {
	resource := ResourceAdaptiveVariable()
	if resource.Importer == nil {
		t.Error("variable must support import")
	}
	if !resource.Schema["name"].ForceNew {
		t.Error("changing variable name must replace it")
	}
	if !resource.Schema["value"].Sensitive {
		t.Error("variable value must be sensitive")
	}
	if got := resource.Schema["labels"].Type; got != schema.TypeSet {
		t.Errorf("labels type = %v, want TypeSet", got)
	}
}
