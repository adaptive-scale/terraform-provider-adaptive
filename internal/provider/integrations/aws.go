package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// AWSCLIIntegrationConfiguration mirrors the backend AWS CLI configuration.
// Role ARN takes precedence over service-account authentication when both are enabled.
type AWSCLIIntegrationConfiguration struct {
	Name                  string `yaml:"name"`
	Version               string `yaml:"version"`
	AWSRegionName         string `yaml:"aws_region_name"`
	AWSAccessKeyID        string `yaml:"aws_access_key_id"`
	AWSSecretAccessKey    string `yaml:"aws_secret_access_key"`
	UseRoleARN            bool   `yaml:"use_role_arn"`
	AWSRoleArn            string `yaml:"aws_role_arn"`
	UseServiceAccount     bool   `yaml:"use_service_account"`
	ServiceAccount        string `yaml:"service_account"`
	CreateIfNotExists     bool   `yaml:"create_if_not_exists"`
	EnableAllowedCommands bool   `yaml:"enable_allowed_commands"`
	AllowedCommands       string `yaml:"allowed_commands"`
}

func SchemaToAWSIntegrationConfiguration(d *schema.ResourceData) AWSCLIIntegrationConfiguration {
	return AWSCLIIntegrationConfiguration{
		Version: "1.0", Name: d.Get("name").(string),
		AWSRegionName:         d.Get("region_name").(string),
		AWSAccessKeyID:        d.Get("access_key_id").(string),
		AWSSecretAccessKey:    d.Get("secret_access_key").(string),
		UseRoleARN:            d.Get("use_role_arn").(bool),
		AWSRoleArn:            d.Get("aws_role_arn").(string),
		UseServiceAccount:     d.Get("use_service_account").(bool),
		ServiceAccount:        d.Get("service_account").(string),
		CreateIfNotExists:     d.Get("create_if_not_exists").(bool),
		EnableAllowedCommands: d.Get("enable_allowed_commands").(bool),
		AllowedCommands:       d.Get("allowed_commands").(string),
	}
}
