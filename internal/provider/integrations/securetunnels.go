package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// SecureTunnelsIntegrationConfiguration carries no settings beyond the name;
// the tunnel is provisioned entirely server-side.
type SecureTunnelsIntegrationConfiguration struct {
	Name string `yaml:"name"`
}

func SchemaToSecureTunnelsIntegrationConfiguration(d *schema.ResourceData) SecureTunnelsIntegrationConfiguration {
	return SecureTunnelsIntegrationConfiguration{
		Name: attrString(d, "name"),
	}
}
