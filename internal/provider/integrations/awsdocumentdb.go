package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AWSDocumentDBIntegrationConfiguration struct {
	Name string `yaml:"name"`
	URI  string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToAWSDocumentDBIntegrationConfiguration(d *schema.ResourceData) AWSDocumentDBIntegrationConfiguration {
	return AWSDocumentDBIntegrationConfiguration{
		Name: d.Get("name").(string),
		URI:  d.Get("uri").(string),
	}
}
