package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MongoAtlasIntegrationConfiguration struct {
	Name           string `yaml:"name"`
	OrganisationID string `yaml:"organization_id"`
	PublicKey      string `yaml:"public_key"`
	PrivateKey     string `yaml:"private_key"`
	ProjectID      string `yaml:"project_id"`
	URI            string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMongoAtlasIntegrationConfiguration(d *schema.ResourceData) MongoAtlasIntegrationConfiguration {
	return MongoAtlasIntegrationConfiguration{
		Name:           d.Get("name").(string),
		URI:            d.Get("uri").(string),
		OrganisationID: d.Get("organization_id").(string),
		PublicKey:      d.Get("public_key").(string),
		PrivateKey:     d.Get("private_key").(string),
		ProjectID:      d.Get("project_id").(string),
	}
}
