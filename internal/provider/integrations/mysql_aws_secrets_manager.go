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

type MySQLAWSIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMySQLAWSIntegrationConfiguration(d *schema.ResourceData) MySQLAWSIntegrationConfiguration {
	return MySQLAWSIntegrationConfiguration{
		Version:  "",
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
	}
}
