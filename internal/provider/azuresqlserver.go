package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AzureSQLServerIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Hostname     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
}

func schemaToAzureSQLServerIntegrationConfiguration(d *schema.ResourceData) AzureSQLServerIntegrationConfiguration {
	return AzureSQLServerIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Hostname:     d.Get("hostname").(string),
		Port:         d.Get("port").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("databaseName").(string),
	}
}
