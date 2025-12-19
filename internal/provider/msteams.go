package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type MSTeamsIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	AppID    string `yaml:"appID"`
	AppKey   string `yaml:"appKey"`
	TenantID string `yaml:"tenantID"`
}

func schemaToMSTeamsIntegrationConfiguration(d *schema.ResourceData) MSTeamsIntegrationConfiguration {
	return MSTeamsIntegrationConfiguration{
		Name:     d.Get("name").(string),
		AppID:    d.Get("client_id").(string),
		AppKey:   d.Get("client_secret").(string),
		TenantID: d.Get("tenant_id").(string),
	}
}
