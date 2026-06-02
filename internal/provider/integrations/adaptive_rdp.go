package integrations

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// AdaptiveRDPTargetConfig mirrors TargetConfig in
// inventorize-app/internal/service/integrations/adaptive_rdp/config.go.
type AdaptiveRDPTargetConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name,omitempty"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Domain   string `yaml:"domain,omitempty"`
	// Record is a tri-state override: nil inherits the global recording
	// setting, true/false force per-target recording on/off.
	Record *bool `yaml:"record,omitempty"`
}

// AdaptiveRDPIntegrationConfiguration mirrors the backend
// AdaptiveRDPIntegrationConfiguration. Targets is a YAML-encoded list of
// AdaptiveRDPTargetConfig that the server re-parses via ParseTargets().
type AdaptiveRDPIntegrationConfiguration struct {
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
	Targets string `yaml:"targets"`
}

func SchemaToAdaptiveRDPIntegrationConfiguration(d *schema.ResourceData) (AdaptiveRDPIntegrationConfiguration, error) {
	raw, ok := d.GetOk("targets")
	if !ok {
		return AdaptiveRDPIntegrationConfiguration{}, errors.New("adaptive_rdp requires at least one `targets` block")
	}
	list, ok := raw.([]interface{})
	if !ok || len(list) == 0 {
		return AdaptiveRDPIntegrationConfiguration{}, errors.New("adaptive_rdp requires at least one `targets` block")
	}

	// `record` is tri-state on the backend (nil = inherit the global
	// COLLECT_RDP_RECORDINGS setting). The SDK collapses an unset bool to
	// false via d.Get, so read the raw config to tell "unset" from "false".
	recordOverrides := adaptiveRDPRecordOverrides(d, len(list))

	targets := make([]AdaptiveRDPTargetConfig, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return AdaptiveRDPIntegrationConfiguration{}, errors.New("could not parse `targets` entry")
		}
		targets = append(targets, AdaptiveRDPTargetConfig{
			ID:       m["id"].(string),
			Name:     m["name"].(string),
			Host:     m["host"].(string),
			Port:     m["port"].(int),
			Username: m["username"].(string),
			Password: m["password"].(string),
			Domain:   m["domain"].(string),
			Record:   recordOverrides[i],
		})
	}

	out, err := yaml.Marshal(targets)
	if err != nil {
		return AdaptiveRDPIntegrationConfiguration{}, fmt.Errorf("could not marshal `targets`: %w", err)
	}

	return AdaptiveRDPIntegrationConfiguration{
		Version: "1.0",
		Name:    d.Get("name").(string),
		Targets: string(out),
	}, nil
}

// adaptiveRDPRecordOverrides returns a per-target *bool for the `record` field,
// distinguishing unset (nil) from an explicit true/false by inspecting the raw
// config — d.Get would report an unset bool as false.
func adaptiveRDPRecordOverrides(d *schema.ResourceData, n int) []*bool {
	out := make([]*bool, n)

	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() {
		return out
	}
	targets := raw.GetAttr("targets")
	if targets.IsNull() || !targets.IsKnown() {
		return out
	}

	elems := targets.AsValueSlice()
	for i := 0; i < n && i < len(elems); i++ {
		e := elems[i]
		if e.IsNull() || !e.IsKnown() {
			continue
		}
		rec := e.GetAttr("record")
		if rec.IsNull() || !rec.IsKnown() || rec.Type() != cty.Bool {
			continue
		}
		v := rec.True()
		out[i] = &v
	}
	return out
}
