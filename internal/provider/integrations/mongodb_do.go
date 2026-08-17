package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type MongoDBDOIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	DBName   string `yaml:"dbname"`
	APIToken string `yaml:"api_token"`
}

func SchemaToMongoDBDOIntegrationConfiguration(d *schema.ResourceData) MongoDBDOIntegrationConfiguration {
	return MongoDBDOIntegrationConfiguration{
		Version:  "1.0",
		Name:     attrString(d, "name"),
		DBName:   attrString(d, "database_name"),
		APIToken: attrString(d, "api_token"),
	}
}
