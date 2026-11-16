package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type KafkaIntegrationConfiguration struct {
	UseMSKIAM           *bool  `yaml:"useMskIam,omitempty"`
	BootstrapServers    string `yaml:"bootstrapServers,omitempty"`
	IAMIdentity         `yaml:",inline"`
	Name                string `yaml:"name"`
	ClientConfiguration string `yaml:"client_configuration"`
}

func SchemaToKafkaIntegrationConfiguration(d *schema.ResourceData) KafkaIntegrationConfiguration {
	return KafkaIntegrationConfiguration{
		UseMSKIAM:           optionalBool(d, "use_msk_iam"),
		BootstrapServers:    attrString(d, "bootstrap_servers"),
		IAMIdentity:         iamIdentity(d, attrBool(d, "use_msk_iam")),
		Name:                attrString(d, "name"),
		ClientConfiguration: attrString(d, "client_configuration"),
	}
}
