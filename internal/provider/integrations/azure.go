package integrations

/*
resource "adaptive_azure" "azure1" {
	name = "thing1"
	tenant_id = ""
	application_id = ""
	client_secret = ""
}
*/
import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AzureIntegrationConfiguration struct {
	Version       string `yaml:"version"`
	Name          string `yaml:"name"`
	TenantID      string `yaml:"tenantID"`
	ApplicationID string `yaml:"applicationID"`
	ClientSecret  string `yaml:"clientSecret"`
}

// schemaToAzureIntegrationConfiguration converts the Terraform schema to an
// AzureIntegrationConfiguration struct that can be serialized as YAML for
// making API calls to the Adaptive Scale platform.
func SchemaToAzureIntegrationConfiguration(d *schema.ResourceData) AzureIntegrationConfiguration {
	return AzureIntegrationConfiguration{
		Version:       "1.0",
		Name:          d.Get("name").(string),
		TenantID:      d.Get("tenant_id").(string),
		ApplicationID: d.Get("application_id").(string),
		ClientSecret:  d.Get("client_secret").(string),
	}
}
