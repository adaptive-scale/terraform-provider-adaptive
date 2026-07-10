package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SnowflakeIntegrationConfiguration struct {
	Name             string `yaml:"name"`
	DatabaseAccount  string `yaml:"databaseAccount"`
	DatabaseUsername string `yaml:"databaseUsername"`
	DatabasePassword string `yaml:"databasePassword"`
	DatabaseName     string `yaml:"databaseName"`
	Warehouse        string `yaml:"warehouse"`
	Schema           string `yaml:"schema"`
	Clientcert       string `yaml:"clientcert"`
	Role             string `yaml:"role"`
}

func SchemaToSnowflakeIntegrationConfiguration(d *schema.ResourceData) SnowflakeIntegrationConfiguration {
	return SnowflakeIntegrationConfiguration{
		Name:             d.Get("name").(string),
		DatabaseAccount:  d.Get("hostname").(string),
		DatabaseUsername: d.Get("username").(string),
		DatabasePassword: d.Get("password").(string),
		DatabaseName:     d.Get("database_name").(string),
		Warehouse:        d.Get("warehouse").(string),
		Schema:           d.Get("schema").(string),
		Clientcert:       d.Get("clientcert").(string),
		Role:             d.Get("role").(string),
	}
}
