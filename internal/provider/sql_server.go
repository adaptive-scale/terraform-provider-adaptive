package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SQLServerIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	DatabaseName string `yaml:"databaseName"`
	Hostname     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
}

func schemaToSQLServerIntegrationConfiguration(d *schema.ResourceData) SQLServerIntegrationConfiguration {
	return SQLServerIntegrationConfiguration{
		Name:         d.Get("name").(string),
		DatabaseName: d.Get("database_name").(string),
		Hostname:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
	}
}
