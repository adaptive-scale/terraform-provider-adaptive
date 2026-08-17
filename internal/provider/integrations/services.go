package integrations

/*
Example resource usage:

resource "adaptive_servicelist" "example" {
	name          = "mydatabase256789"
	urls      = "comma,separated,urls"
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ServiceListIntegrationConfiguration struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	URLs    string `yaml:"urls"`
}

func SchemaToServiceListIntegrationConfiguration(d *schema.ResourceData) ServiceListIntegrationConfiguration {
	return ServiceListIntegrationConfiguration{
		Version: "1",
		Name:    d.Get("name").(string),
		URLs:    d.Get("urls").(string),
	}
}
