package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AzureSQLServerIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Hostname     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
}

func SchemaToAzureSQLServerIntegrationConfiguration(d *schema.ResourceData) AzureSQLServerIntegrationConfiguration {
	// Validate required fields
	name, ok := d.Get("name").(string)
	if !ok || name == "" {
		panic("name attribute is required and must be a non-empty string")
	}

	hostname, ok := d.Get("hostname").(string)
	if !ok || hostname == "" {
		panic("hostname attribute is required and must be a non-empty string")
	}

	port, ok := d.Get("port").(string)
	if !ok || port == "" {
		panic("port attribute is required and must be a non-empty string")
	}

	username, ok := d.Get("username").(string)
	if !ok || username == "" {
		panic("username attribute is required and must be a non-empty string")
	}

	password, ok := d.Get("password").(string)
	if !ok || password == "" {
		panic("password attribute is required and must be a non-empty string")
	}

	databaseName, ok := d.Get("database_name").(string)
	if !ok || databaseName == "" {
		panic("database_name attribute is required and must be a non-empty string")
	}

	return AzureSQLServerIntegrationConfiguration{
		Name:         name,
		Hostname:     hostname,
		Port:         port,
		Username:     username,
		Password:     password,
		DatabaseName: databaseName,
	}
}
