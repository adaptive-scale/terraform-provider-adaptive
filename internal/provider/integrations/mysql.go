package integrations

/*
Example resource usage:

resource "adaptive_mysql" "example" {
	  name          = "mydatabase256789"
	  database_name = ""
	  host          = "myhost.example.com"
	  port          = "5433"
	  username      = "myuser"
	  password      = "mypasswor2"
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MySQLIntegrationConfiguration struct {
	Version      string `yaml:"version"`
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMySQLIntegrationConfiguration(d *schema.ResourceData) MySQLIntegrationConfiguration {
	return MySQLIntegrationConfiguration{
		Version:      "",
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      "require",
	}
}
