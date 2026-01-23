package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AzureActiveDirectoryIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	TenantID     string `yaml:"tenantID"`
	UseTenant    bool   `yaml:"useTenant"`
}

// SchemaToAzureActiveDirectoryIntegrationConfiguration converts the Terraform schema to an
// AzureActiveDirectoryIntegrationConfiguration struct that can be serialized as YAML for
// making API calls to the Adaptive Scale platform.
func SchemaToAzureActiveDirectoryIntegrationConfiguration(d *schema.ResourceData) AzureActiveDirectoryIntegrationConfiguration {
	return AzureActiveDirectoryIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
		TenantID:     d.Get("tenant_id").(string),
		UseTenant:    d.Get("use_tenant").(bool),
	}
}
