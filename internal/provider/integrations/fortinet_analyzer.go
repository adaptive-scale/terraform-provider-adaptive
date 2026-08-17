package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func SchemaToFortinetAnalyzerIntegrationConfiguration(d *schema.ResourceData) FortinetApplianceIntegrationConfiguration {
	return schemaToFortinetApplianceConfiguration(d)
}
