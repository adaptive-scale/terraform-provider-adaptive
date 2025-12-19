package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type CustomSIEMWebhookIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Url          string `yaml:"url"`
	SharedSecret string `yaml:"sharedSecret,omitempty"`
}

func schemaToCustomSIEMWebhookIntegrationConfiguration(d *schema.ResourceData) CustomSIEMWebhookIntegrationConfiguration {
	return CustomSIEMWebhookIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Url:          d.Get("uri").(string),
		SharedSecret: d.Get("shared_secret").(string),
	}
}
