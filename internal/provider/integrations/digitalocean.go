package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type DigitalOceanIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	ApiToken string `yaml:"api_token"`
}

func SchemaToDigitalOceanIntegrationConfiguration(d *schema.ResourceData) DigitalOceanIntegrationConfiguration {
	return DigitalOceanIntegrationConfiguration{
		Version:  "1.0",
		Name:     attrString(d, "name"),
		ApiToken: attrString(d, "api_token"),
	}
}
