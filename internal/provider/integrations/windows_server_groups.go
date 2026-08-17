package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type WindowsServerGroupsIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	Hosts    string `yaml:"hosts"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Port     string `yaml:"port"`
}

func SchemaToWindowsServerGroupsIntegrationConfiguration(d *schema.ResourceData) WindowsServerGroupsIntegrationConfiguration {
	return WindowsServerGroupsIntegrationConfiguration{
		Version:  "1.0",
		Name:     attrString(d, "name"),
		Hosts:    strings.Join(attrStringList(d, "hosts"), "\n"),
		Username: attrString(d, "username"),
		Password: attrString(d, "password"),
		Port:     attrString(d, "port"),
	}
}
