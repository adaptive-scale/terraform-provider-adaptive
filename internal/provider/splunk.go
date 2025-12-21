package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SplunkIntegrationConfiguration struct {
	Name    string `yaml:"name"`
	TokenID string `yaml:"tokenID"`
	Url     string `yaml:"url"`
}

func schemaToSplunkIntegrationConfiguration(d *schema.ResourceData) SplunkIntegrationConfiguration {
	return SplunkIntegrationConfiguration{
		Name:    d.Get("name").(string),
		TokenID: d.Get("token_id").(string),
		Url:     d.Get("url").(string),
	}
}
