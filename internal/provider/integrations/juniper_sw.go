package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func SchemaToJuniperSWIntegrationConfiguration(d *schema.ResourceData) SSHApplianceIntegrationConfiguration {
	return schemaToSSHApplianceConfiguration(d)
}
