package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ZeroTierConfiguration struct {
	Name      string `yaml:"name"`
	NetworkID string `yaml:"network_id"`
	Token     string `yaml:"api_token,omitempty"`
	Version   string `yaml:"version,omitempty"`
}

func schemaToZeroTierIntegrationConfiguration(d *schema.ResourceData) ZeroTierConfiguration {
	return ZeroTierConfiguration{
		Version:   "1.0",
		Name:      d.Get("name").(string),
		NetworkID: d.Get("network_id").(string),
		Token:     d.Get("api_token").(string),
	}
}
