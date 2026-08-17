package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ConfluentIntegrationConfiguration struct {
	Version        string `yaml:"version"`
	Name           string `yaml:"name"`
	OrganizationID string `yaml:"organization_id"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	Resource       string `yaml:"resource"`
	APIKey         string `yaml:"apiKey"`
	APISecret      string `yaml:"apiSecret"`
}

func SchemaToConfluentIntegrationConfiguration(d *schema.ResourceData) ConfluentIntegrationConfiguration {
	return ConfluentIntegrationConfiguration{
		Version:        "1.0",
		Name:           attrString(d, "name"),
		OrganizationID: attrString(d, "organization_id"),
		Username:       attrString(d, "username"),
		Password:       attrString(d, "password"),
		Resource:       attrString(d, "resource"),
		APIKey:         attrString(d, "api_key"),
		APISecret:      attrString(d, "api_secret"),
	}
}
