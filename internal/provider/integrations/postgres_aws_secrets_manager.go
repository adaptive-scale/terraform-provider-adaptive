package integrations

/*
   Example resource usage:

   resource "adaptive_postgres" "example" {
     name          = "mydatabase256789"
     host          = "myhost.example.com"
     port          = "5433"
     username      = "myuser"
     password      = "mypasswor2"
     database_name = ""
   }

*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type PostgresIntegrationAWSConfiguration struct {
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToPostgresAWSIntegrationConfiguration(d *schema.ResourceData) PostgresIntegrationAWSConfiguration {
	return PostgresIntegrationAWSConfiguration{
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
	}
}
