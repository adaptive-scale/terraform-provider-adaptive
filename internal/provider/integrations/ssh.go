package integrations

/*
Example resource usage:

	resource "adaptive_ssh" "example" {
		name = "instance-name"
		username = "myuser"
		hostname = "myhost.example.com"
		port = "22"
		key = ""
	}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SSHIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	UsePassword bool   `yaml:"usePassword"`
	Password    string `yaml:"password"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	SSHKey      string `yaml:"sshKey"`
}

func SchemaToSSHIntegrationConfiguration(d *schema.ResourceData) SSHIntegrationConfiguration {
	return SSHIntegrationConfiguration{
		Version:     "1.0",
		Name:        d.Get("name").(string),
		Username:    d.Get("username").(string),
		UsePassword: d.Get("key").(string) == "",
		Password:    d.Get("key").(string),
		HostName:    d.Get("host").(string),
		Port:        d.Get("port").(string),
		SSHKey:      d.Get("key").(string),
	}
}
