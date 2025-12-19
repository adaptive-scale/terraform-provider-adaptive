package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type DatadogIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	DdSite   string `yaml:"dd_site"`
	DdApiKey string `yaml:"dd_api_key"`
}

func schemaToDatadogIntegrationConfiguration(d *schema.ResourceData) DatadogIntegrationConfiguration {
	return DatadogIntegrationConfiguration{
		Name:     d.Get("name").(string),
		DdSite:   d.Get("dd_site").(string),
		DdApiKey: d.Get("dd_api_key").(string),
	}
}
