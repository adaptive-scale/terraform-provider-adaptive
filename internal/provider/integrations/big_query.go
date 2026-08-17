package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type BigQueryIntegrationConfiguration struct {
	Version        string `yaml:"version"`
	Name           string `yaml:"name"`
	CredentialJSON string `yaml:"credential_json"`
}

func SchemaToBigQueryIntegrationConfiguration(d *schema.ResourceData) BigQueryIntegrationConfiguration {
	return BigQueryIntegrationConfiguration{
		Version:        "1.0",
		Name:           attrString(d, "name"),
		CredentialJSON: strings.TrimSpace(attrString(d, "credential_json")),
	}
}
