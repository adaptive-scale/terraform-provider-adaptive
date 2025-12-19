package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SyslogIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Port     string `yaml:"port"`
	Protocol string `yaml:"protocol"`
}

func schemaToSyslogIntegrationConfiguration(d *schema.ResourceData) SyslogIntegrationConfiguration {
	return SyslogIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Hostname: d.Get("hostname").(string),
		Port:     d.Get("port").(string),
		Protocol: d.Get("protocol").(string),
	}
}
