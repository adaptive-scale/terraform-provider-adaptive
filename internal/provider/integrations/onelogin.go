package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type OneLoginIntegrationConfiguration struct {
	Name            string `yaml:"name"`
	Domain          string `yaml:"domain"`
	ClientID        string `yaml:"clientID"`
	ClientSecret    string `yaml:"clientSecret"`
	ApiClientID     string `yaml:"apiClientID"`
	ApiClientSecret string `yaml:"apiClientSecret"`
}

func SchemaToOneLoginIntegrationConfiguration(d *schema.ResourceData) OneLoginIntegrationConfiguration {
	return OneLoginIntegrationConfiguration{
		Name:            d.Get("name").(string),
		Domain:          d.Get("domain").(string),
		ClientID:        d.Get("client_id").(string),
		ClientSecret:    d.Get("client_secret").(string),
		ApiClientID:     d.Get("api_client_id").(string),
		ApiClientSecret: d.Get("api_client_secret").(string),
	}
}
