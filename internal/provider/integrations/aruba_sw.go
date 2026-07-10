package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ArubaSWIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func SchemaToArubaSWIntegrationConfiguration(d *schema.ResourceData) ArubaSWIntegrationConfiguration {
	return ArubaSWIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Hostname: d.Get("hostname").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
	}
}
