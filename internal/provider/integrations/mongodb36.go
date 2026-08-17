package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MongoDB36IntegrationConfiguration struct {
	Version           string `yaml:"version"`
	Name              string `yaml:"name"`
	URI               string `yaml:"uri"`
	ClientCertificate string `yaml:"clientCertificate,omitempty"`
	UseTLS            bool   `yaml:"useTLS,omitempty"`
}

func SchemaToMongoDB36IntegrationConfiguration(d *schema.ResourceData) MongoDB36IntegrationConfiguration {
	return MongoDB36IntegrationConfiguration{
		Version:           "1.0",
		Name:              attrString(d, "name"),
		URI:               attrString(d, "uri"),
		ClientCertificate: strings.TrimSpace(attrString(d, "client_certificate")),
		UseTLS:            attrBool(d, "use_tls"),
	}
}
