package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SQLServerAWSIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
}

func schemaToSQLServerAWSIntegrationConfiguration(d *schema.ResourceData) SQLServerAWSIntegrationConfiguration {
	return SQLServerAWSIntegrationConfiguration{
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
	}
}
