package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AzureCosmosNoSQLIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Key      string `yaml:"key"`
}

func schemaToAzureCosmosNoSQLIntegrationConfiguration(d *schema.ResourceData) AzureCosmosNoSQLIntegrationConfiguration {
	return AzureCosmosNoSQLIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Endpoint: d.Get("uri").(string),
		Key:      d.Get("api_token").(string),
	}
}
