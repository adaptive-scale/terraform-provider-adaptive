package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type DatadogIntegrationConfiguration struct {
	DdAppKey string `yaml:"dd_app_key"`
	Name     string `yaml:"name"`
	DdSite   string `yaml:"dd_site"`
	DdApiKey string `yaml:"dd_api_key"`
}

func SchemaToDatadogIntegrationConfiguration(d *schema.ResourceData) DatadogIntegrationConfiguration {
	return DatadogIntegrationConfiguration{
		DdAppKey: d.Get("app_key").(string),
		Name:     d.Get("name").(string),
		DdSite:   d.Get("dd_site").(string),
		DdApiKey: d.Get("dd_api_key").(string),
	}
}
