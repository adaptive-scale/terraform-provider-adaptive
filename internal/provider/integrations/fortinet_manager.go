package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func SchemaToFortinetManagerIntegrationConfiguration(d *schema.ResourceData) FortinetApplianceIntegrationConfiguration {
	return schemaToFortinetApplianceConfiguration(d)
}
