package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type OnePasswordIntegrationConfiguration struct {
	Name             string `yaml:"name"`
	ServiceAccount   string `yaml:"service_account"`
	UseConnectServer bool   `yaml:"use_connect_server"`
	ConnectServerURL string `yaml:"connect_server_url"`
}

func SchemaToOnePasswordIntegrationConfiguration(d *schema.ResourceData) OnePasswordIntegrationConfiguration {
	return OnePasswordIntegrationConfiguration{
		Name:             attrString(d, "name"),
		ServiceAccount:   attrString(d, "service_account"),
		UseConnectServer: attrBool(d, "use_connect_server"),
		ConnectServerURL: attrString(d, "connect_server_url"),
	}
}
