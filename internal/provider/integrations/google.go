package integrations

/*
resource "adaptive_google" "example" {
	name = "instance-name"
	domain = ""
	client_id = ""
	client_secret = ""
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type GoogleOAuthIntegrationConfiguration struct {
	Version      string `yaml:"Version"`
	Name         string `yaml:"name"`
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

func SchemaToGoogleOAuthIntegrationConfiguration(d *schema.ResourceData) GoogleOAuthIntegrationConfiguration {
	return GoogleOAuthIntegrationConfiguration{
		Version:      "1",
		Name:         d.Get("name").(string),
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
	}
}
