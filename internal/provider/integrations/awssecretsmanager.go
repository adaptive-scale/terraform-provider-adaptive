package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSSecretsManagerConfiguration struct {
	Version       string `yaml:"version"`
	Name          string `yaml:"name"`
	AWSRegionName string `yaml:"aws_region_name"`
	AWSARN        string `yaml:"aws_arn"`
}

func SchemaToAWSSecretsManagerConfiguration(d *schema.ResourceData) AWSSecretsManagerConfiguration {
	return AWSSecretsManagerConfiguration{
		Name:          d.Get("name").(string),
		AWSRegionName: d.Get("aws_region_name").(string),
		AWSARN:        d.Get("aws_arn").(string),
	}
}
