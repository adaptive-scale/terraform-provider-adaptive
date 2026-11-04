package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MongoIntegrationConfiguration struct {
	ClientCertificate string `yaml:"clientCertificate"`
	UseTLS            bool   `yaml:"useTLS"`
	Version           string `yaml:"version"`

	Name string `yaml:"name"`
	URI  string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMongoIntegrationConfiguration(d *schema.ResourceData) MongoIntegrationConfiguration {
	return MongoIntegrationConfiguration{
		ClientCertificate: attrString(d, "client_certificate"),
		UseTLS:            attrBool(d, "use_tls"),
		Version:           attrString(d, "version"),
		Name:              d.Get("name").(string),
		URI:               d.Get("uri").(string),
	}
}
