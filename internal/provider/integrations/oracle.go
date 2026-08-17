package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type OracleIntegrationConfiguration struct {
	Name        string `yaml:"name"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	ServiceName string `yaml:"service_name"`
}

func SchemaToOracleIntegrationConfiguration(d *schema.ResourceData) OracleIntegrationConfiguration {
	return OracleIntegrationConfiguration{
		Name:        attrString(d, "name"),
		HostName:    attrString(d, "host"),
		Port:        attrString(d, "port"),
		Username:    attrString(d, "username"),
		Password:    attrString(d, "password"),
		ServiceName: attrString(d, "service_name"),
	}
}
