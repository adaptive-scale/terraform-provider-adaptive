package components

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func TestManagedDeviceExplicitSecretClearsRemainInPayload(t *testing.T) {
	for _, typ := range []string{"aruba_instant_on", "aruba_sw", "cisco_ngfw", "fortinet_ngfw", "fortinet_analyzer", "fortinet_manager", "hpe_switch", "ivanti", "juniper_sw", "paloalto_ngfw", "sophos_fw"} {
		t.Run(typ, func(t *testing.T) {
			r := ResourceAdaptiveResource()
			d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "fixture", "type": typ, "fields": "", "script": ""})
			d.SetId("resource-1")
			state := d.State()
			state.RawConfig = cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("fixture"), "type": cty.StringVal(typ), "fields": cty.StringVal(""), "script": cty.StringVal("")})
			planned, err := schema.InternalMap(r.Schema).Data(state, nil)
			if err != nil {
				t.Fatal(err)
			}
			obj, err := schemaToResourceIntegrationConfiguration(planned, typ)
			if err != nil {
				t.Fatal(err)
			}
			data, err := yaml.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]interface{}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"fields", "script"} {
				if value, present := cfg[key]; !present || value != "" {
					t.Errorf("explicit %s clear must serialize an authoritative empty string", key)
				}
			}
		})
	}
}
