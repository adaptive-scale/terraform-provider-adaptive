package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type RDPWindowsIntegrationConfiguration struct {
	AllowFileTransfer bool   `yaml:"allowFileTransfer"`
	Version           string `yaml:"version"`
	Name              string `yaml:"name"`
	Hostname          string `yaml:"hostname"`
	Password          string `yaml:"password"`
	Username          string `yaml:"username"`
	Port              string `yaml:"port"`
}

func SchemaToRDPWindowsIntegrationConfiguration(d *schema.ResourceData) RDPWindowsIntegrationConfiguration {
	return RDPWindowsIntegrationConfiguration{
		AllowFileTransfer: attrBool(d, "allow_file_transfer"),
		Version:           "1.0",
		Name:              d.Get("name").(string),
		Hostname:          d.Get("hostname").(string),
		Password:          d.Get("password").(string),
		Username:          d.Get("username").(string),
		Port:              d.Get("port").(string),
	}
}
