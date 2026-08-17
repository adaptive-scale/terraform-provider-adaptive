package integrations

/*
Example resource usage:

resource "adaptive_aws" "example" {
	  aws_region_name = "us-east-1"
	  aws_access_key_id = "myaccesskey"
	  aws_secret_access_key = "kysecretaccesskey"
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AWSCLIIntegrationConfiguration struct {
	Name               string `yaml:"name"`
	Version            string `yaml:"version"`
	AWSRegionName      string `yaml:"aws_region_name"`
	AWSAccessKeyID     string `yaml:"aws_access_key_id"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key"`
}

func SchemaToAWSIntegrationConfiguration(d *schema.ResourceData) AWSCLIIntegrationConfiguration {
	return AWSCLIIntegrationConfiguration{
		Version:            "1.0",
		Name:               d.Get("name").(string),
		AWSRegionName:      d.Get("region_name").(string),
		AWSAccessKeyID:     d.Get("access_key_id").(string),
		AWSSecretAccessKey: d.Get("secret_access_key").(string),
	}
}
