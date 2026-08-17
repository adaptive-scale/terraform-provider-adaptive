package integrations

/*
Example resource usage:

resource "adaptive_okta" "example" {
	name = "instance-name"
	domain = ""
	client_id = ""
	client_secret = ""
}
*/
import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type OktaOAuthIntegrationConfiguration struct {
	Version      string `yaml:"version"`
	Name         string `yaml:"name"`
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToOktaIntegrationConfiguration(d *schema.ResourceData) OktaOAuthIntegrationConfiguration {
	return OktaOAuthIntegrationConfiguration{
		Version:      "1.0",
		Name:         d.Get("name").(string),
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
	}
}
