package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type VNCIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Hosts    string `yaml:"hosts"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
}

func SchemaToVNCIntegrationConfiguration(d *schema.ResourceData) VNCIntegrationConfiguration {
	return VNCIntegrationConfiguration{
		Name:     attrString(d, "name"),
		Hosts:    strings.Join(attrStringList(d, "hosts"), "\n"),
		Port:     attrString(d, "port"),
		Password: attrString(d, "password"),
	}
}
