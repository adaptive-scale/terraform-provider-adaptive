package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ClickHouseIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"database"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	UseTLS       bool   `yaml:"use-tls"`
}

func SchemaToClickHouseIntegrationConfiguration(d *schema.ResourceData) ClickHouseIntegrationConfiguration {
	return ClickHouseIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		UseTLS:       d.Get("use_tls").(bool),
	}
}
