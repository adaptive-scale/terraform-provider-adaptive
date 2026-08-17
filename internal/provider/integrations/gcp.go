package integrations

/*
resource "adaptive_gcp" "example" {
	name = "instance-name"
	project_id = ""
	key_file = ""
*/

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type GCPIntegrationConfiguration struct {
	Version   string `yaml:"version"`
	Name      string `yaml:"name"`
	ProjectID string `yaml:"project_id"`
	KeyFile   string `yaml:"key_file"`
}

func SchemaToGCPIntegrationConfiguration(d *schema.ResourceData) GCPIntegrationConfiguration {
	return GCPIntegrationConfiguration{
		Version:   "1",
		Name:      d.Get("name").(string),
		ProjectID: d.Get("project_id").(string),
		KeyFile:   strings.TrimSpace(d.Get("key_file").(string)),
	}
}
