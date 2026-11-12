package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The fixture records the public schema before v0.9 API alignment. New fields
// are allowed; existing HCL/state types, defaults and replacement rules must
// remain unchanged. Descriptions and validation function addresses are omitted.
func TestLegacySchemaCompatibility(t *testing.T) {
	fixture, err := os.ReadFile("testdata/legacy_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var previous map[string]string
	if err := json.Unmarshal(fixture, &previous); err != nil {
		t.Fatal(err)
	}
	current := schemaContract(New("test")())
	for path, want := range previous {
		if got := current[path]; got != want {
			t.Errorf("legacy schema changed at %s: got %s, want %s", path, got, want)
		}
	}
}

func schemaContract(p *schema.Provider) map[string]string {
	out := map[string]string{}
	var visit func(string, map[string]*schema.Schema)
	visit = func(prefix string, fields map[string]*schema.Schema) {
		for name, field := range fields {
			path := prefix + "." + name
			defaults, _ := json.Marshal(field.Default)
			out[path] = fmt.Sprintf("%s required=%v optional=%v computed=%v forceNew=%v sensitive=%v default=%s min=%d max=%d", field.Type, field.Required, field.Optional, field.Computed, field.ForceNew, field.Sensitive, defaults, field.MinItems, field.MaxItems)
			switch elem := field.Elem.(type) {
			case *schema.Resource:
				visit(path, elem.Schema)
			case *schema.Schema:
				visit(path, map[string]*schema.Schema{"element": elem})
			}
		}
	}
	visit("provider", p.Schema)
	for name, resource := range p.ResourcesMap {
		out[name+".schema_version"] = fmt.Sprint(resource.SchemaVersion)
		visit(name, resource.Schema)
	}
	return out
}
