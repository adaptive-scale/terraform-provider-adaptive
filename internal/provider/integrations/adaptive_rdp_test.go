package integrations

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func adaptiveRDPTestSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {Type: schema.TypeString, Optional: true},
		"targets": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id":       {Type: schema.TypeString, Required: true},
					"name":     {Type: schema.TypeString, Optional: true},
					"host":     {Type: schema.TypeString, Required: true},
					"port":     {Type: schema.TypeInt, Optional: true, Default: 3389},
					"username": {Type: schema.TypeString, Required: true},
					"password": {Type: schema.TypeString, Optional: true, Sensitive: true},
					"domain":   {Type: schema.TypeString, Optional: true},
					"record":   {Type: schema.TypeBool, Optional: true},
				},
			},
		},
	}
}

func TestSchemaToAdaptiveRDPIntegrationConfiguration_NoTargets(t *testing.T) {
	d := schema.TestResourceDataRaw(t, adaptiveRDPTestSchema(), map[string]interface{}{
		"name": "rdp-fleet",
	})

	if _, err := SchemaToAdaptiveRDPIntegrationConfiguration(d); err == nil {
		t.Fatal("expected an error when no targets are configured")
	}
}

func TestSchemaToAdaptiveRDPIntegrationConfiguration_DuplicateID(t *testing.T) {
	d := schema.TestResourceDataRaw(t, adaptiveRDPTestSchema(), map[string]interface{}{
		"name": "rdp-fleet",
		"targets": []interface{}{
			map[string]interface{}{"id": "server-1", "host": "10.0.0.5", "username": "admin"},
			map[string]interface{}{"id": "server-1", "host": "10.0.0.6", "username": "admin"},
		},
	})

	_, err := SchemaToAdaptiveRDPIntegrationConfiguration(d)
	if err == nil {
		t.Fatal("expected an error for duplicate target ids")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected a duplicate-id error, got: %v", err)
	}
}

func TestSchemaToAdaptiveRDPIntegrationConfiguration_Success(t *testing.T) {
	d := schema.TestResourceDataRaw(t, adaptiveRDPTestSchema(), map[string]interface{}{
		"name": "rdp-fleet",
		"targets": []interface{}{
			map[string]interface{}{"id": "server-1", "host": "10.0.0.5", "username": "admin"},
			map[string]interface{}{"id": "server-2", "host": "10.0.0.6", "username": "admin"},
		},
	})

	cfg, err := SchemaToAdaptiveRDPIntegrationConfiguration(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cfg.Targets, "server-1") || !strings.Contains(cfg.Targets, "server-2") {
		t.Fatalf("expected marshaled targets to contain both ids, got: %s", cfg.Targets)
	}
}

// TestAdaptiveRDPRecordOverrides_TriState exercises the raw-config/cty
// traversal that distinguishes an explicit `record = false` from an unset
// `record` (which d.Get alone cannot do, since both collapse to false).
// schema.TestResourceDataRaw does not populate GetRawConfig(), so the
// ResourceData is built by hand here with a crafted RawConfig.
func TestAdaptiveRDPRecordOverrides_TriState(t *testing.T) {
	targetObj := func(id string, record cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"id":       cty.StringVal(id),
			"name":     cty.NullVal(cty.String),
			"host":     cty.StringVal("host"),
			"port":     cty.NumberIntVal(3389),
			"username": cty.StringVal("user"),
			"password": cty.NullVal(cty.String),
			"domain":   cty.NullVal(cty.String),
			"record":   record,
		})
	}

	rawConfig := cty.ObjectVal(map[string]cty.Value{
		"targets": cty.ListVal([]cty.Value{
			targetObj("explicit-true", cty.True),
			targetObj("unset", cty.NullVal(cty.Bool)),
			targetObj("explicit-false", cty.False),
		}),
	})

	sm := schema.InternalMap(adaptiveRDPTestSchema())
	d, err := sm.Data(nil, &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{},
		RawConfig:  rawConfig,
	})
	if err != nil {
		t.Fatalf("failed to build resource data: %v", err)
	}

	overrides := adaptiveRDPRecordOverrides(d, 3)

	if overrides[0] == nil || *overrides[0] != true {
		t.Errorf("expected index 0 (explicit true) to be *true, got %v", overrides[0])
	}
	if overrides[1] != nil {
		t.Errorf("expected index 1 (unset) to be nil, got %v", *overrides[1])
	}
	if overrides[2] == nil || *overrides[2] != false {
		t.Errorf("expected index 2 (explicit false) to be *false, got %v", overrides[2])
	}
}

func TestAdaptiveRDPRecordOverrides_NoRawConfig(t *testing.T) {
	d := schema.TestResourceDataRaw(t, adaptiveRDPTestSchema(), map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{"id": "server-1", "host": "10.0.0.5", "username": "admin"},
		},
	})

	overrides := adaptiveRDPRecordOverrides(d, 1)
	if overrides[0] != nil {
		t.Errorf("expected nil override when raw config is unavailable, got %v", *overrides[0])
	}
}
