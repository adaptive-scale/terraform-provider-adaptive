package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type CustomIntegrationConfiguration struct {
	Name               string `yaml:"name"`
	Image              string `yaml:"image"`
	ServiceAccountName string `yaml:"service_account_name,omitempty"`
}

func schemaToCustomIntegrationConfiguration(d *schema.ResourceData) CustomIntegrationConfiguration {
	return CustomIntegrationConfiguration{
		Name:               d.Get("name").(string),
		Image:              d.Get("image").(string),
		ServiceAccountName: d.Get("service_account_name").(string),
	}
}
