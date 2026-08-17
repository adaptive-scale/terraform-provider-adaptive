package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AvastBusinessEDRIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

func SchemaToAvastBusinessEDRIntegrationConfiguration(d *schema.ResourceData) AvastBusinessEDRIntegrationConfiguration {
	return AvastBusinessEDRIntegrationConfiguration{
		Name:         attrString(d, "name"),
		ClientID:     attrString(d, "client_id"),
		ClientSecret: attrString(d, "client_secret"),
	}
}
