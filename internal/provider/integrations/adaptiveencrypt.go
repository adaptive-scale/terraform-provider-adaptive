package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AdaptiveEncryptIntegrationConfiguration struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func SchemaToAdaptiveEncryptIntegrationConfiguration(d *schema.ResourceData) AdaptiveEncryptIntegrationConfiguration {
	return AdaptiveEncryptIntegrationConfiguration{
		Name:  attrString(d, "name"),
		Value: attrString(d, "value"),
	}
}
