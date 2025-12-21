package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type CiscoNGFWIntegrationConfiguration struct {
	Name      string `yaml:"name"`
	Hostname  string `yaml:"hostname"`
	LoginUrl  string `yaml:"login_url"`
	Port      string `yaml:"port"`
	Type      string `yaml:"type"`
	UseProxy  bool   `yaml:"use_proxy"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Version   string `yaml:"version"`
	WebuiPort string `yaml:"webui_port"`
}

func schemaToCiscoNGFWIntegrationConfiguration(d *schema.ResourceData) CiscoNGFWIntegrationConfiguration {
	return CiscoNGFWIntegrationConfiguration{
		Hostname:  d.Get("hostname").(string),
		LoginUrl:  d.Get("uri").(string),
		Name:      d.Get("name").(string),
		Port:      d.Get("port").(string),
		UseProxy:  d.Get("use_proxy").(bool),
		Username:  d.Get("username").(string),
		Password:  d.Get("password").(string),
		WebuiPort: d.Get("webui_port").(string),
	}
}
