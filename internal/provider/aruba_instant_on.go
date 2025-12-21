package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ArubaInstantOnIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	APIToken string `yaml:"apiToken"`
}

func schemaToArubaInstantOnIntegrationConfiguration(d *schema.ResourceData) ArubaInstantOnIntegrationConfiguration {
	return ArubaInstantOnIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Host:     d.Get("host").(string),
		Port:     d.Get("port").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
		APIToken: d.Get("api_token").(string),
	}
}
