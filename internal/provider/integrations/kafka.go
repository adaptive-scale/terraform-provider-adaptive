package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type KafkaIntegrationConfiguration struct {
	Name                string `yaml:"name"`
	ClientConfiguration string `yaml:"client_configuration"`
}

func SchemaToKafkaIntegrationConfiguration(d *schema.ResourceData) KafkaIntegrationConfiguration {
	return KafkaIntegrationConfiguration{
		Name:                attrString(d, "name"),
		ClientConfiguration: attrString(d, "client_configuration"),
	}
}
