package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// AdaptiveRemoteDesktopIntegrationConfiguration uses the lowercase keys the
// remote desktop integration unmarshals (cpu/memory/storage), not the
// capitalised form labels: yaml.v2 matches struct tags case-sensitively.
type AdaptiveRemoteDesktopIntegrationConfiguration struct {
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
	Cpu     string `yaml:"cpu"`
	Memory  string `yaml:"memory"`
	Storage string `yaml:"storage"`
}

func SchemaToAdaptiveRemoteDesktopIntegrationConfiguration(d *schema.ResourceData) AdaptiveRemoteDesktopIntegrationConfiguration {
	return AdaptiveRemoteDesktopIntegrationConfiguration{
		Version: "1.0",
		Name:    attrString(d, "name"),
		Cpu:     attrString(d, "cpu"),
		Memory:  attrString(d, "memory"),
		Storage: attrString(d, "storage"),
	}
}
