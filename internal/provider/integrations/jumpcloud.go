package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type JumpCloudIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	Domain       string `yaml:"domain"`
	ApiKey       string `yaml:"apiKey"`
}

func SchemaToJumpCloudIntegrationConfiguration(d *schema.ResourceData) JumpCloudIntegrationConfiguration {
	return JumpCloudIntegrationConfiguration{
		Name:         d.Get("name").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
		Domain:       d.Get("domain").(string),
		ApiKey:       d.Get("api_token").(string),
	}
}
