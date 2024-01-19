package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSSecretsManagerConfiguration struct {
	Version       string `yaml:"version"`
	AWSRegionName string `yaml:"aws_region_name"`
	AWSARN        string `yaml:"aws_arn"`
}

func schemaToAWSSecretsManagerConfiguration(d *schema.ResourceData) AWSSecretsManagerConfiguration {
	return AWSSecretsManagerConfiguration{
		AWSRegionName: d.Get("aws_region_name").(string),
		AWSARN:        d.Get("aws_arn").(string),
	}
}
