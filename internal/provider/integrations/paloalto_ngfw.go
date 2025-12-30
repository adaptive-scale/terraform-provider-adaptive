package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type PaloAltoNGFWIntegrationConfiguration struct {
	Name      string `yaml:"name"`
	Password  string `yaml:"password"`
	Username  string `yaml:"username"`
	Hostname  string `yaml:"hostname"`
	WebuiPort string `yaml:"webui_port"`
	LoginUrl  string `yaml:"login_url"`
}

func SchemaToPaloAltoNGFWIntegrationConfiguration(d *schema.ResourceData) PaloAltoNGFWIntegrationConfiguration {
	return PaloAltoNGFWIntegrationConfiguration{
		Name:      d.Get("name").(string),
		Password:  d.Get("password").(string),
		Username:  d.Get("username").(string),
		Hostname:  d.Get("hostname").(string),
		WebuiPort: d.Get("webui_port").(string),
		LoginUrl:  d.Get("login_url").(string),
	}
}
