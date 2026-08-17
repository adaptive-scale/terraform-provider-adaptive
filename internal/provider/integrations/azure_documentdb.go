package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AzureDocumentDBIntegrationConfiguration struct {
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
	URI     string `yaml:"uri"`
}

func SchemaToAzureDocumentDBIntegrationConfiguration(d *schema.ResourceData) AzureDocumentDBIntegrationConfiguration {
	return AzureDocumentDBIntegrationConfiguration{
		Version: "1.0",
		Name:    attrString(d, "name"),
		URI:     attrString(d, "uri"),
	}
}
