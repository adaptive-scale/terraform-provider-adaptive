package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type KeyspacesIntegrationConfiguration struct {
	UseServiceAccount bool   `yaml:"use_service_account"`
	CreateIfNotExists bool   `yaml:"create_if_not_exists"`
	Name              string `yaml:"name"`
}

func schemaToKeyspacesIntegrationConfiguration(d *schema.ResourceData) KeyspacesIntegrationConfiguration {
	return KeyspacesIntegrationConfiguration{
		UseServiceAccount: d.Get("use_service_account").(bool),
		CreateIfNotExists: d.Get("create_if_not_exists").(bool),
		Name:              d.Get("name").(string),
	}
}
