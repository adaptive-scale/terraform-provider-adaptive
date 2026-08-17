package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MongoIntegrationConfiguration struct {
	Name string `yaml:"name"`
	URI  string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMongoIntegrationConfiguration(d *schema.ResourceData) MongoIntegrationConfiguration {
	return MongoIntegrationConfiguration{
		Name: d.Get("name").(string),
		URI:  d.Get("uri").(string),
	}
}
