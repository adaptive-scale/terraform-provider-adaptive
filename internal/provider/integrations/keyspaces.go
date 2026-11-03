package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type KeyspacesIntegrationConfiguration struct {
	Name              string `yaml:"name"`
	Keyspace          string `yaml:"keyspace"`
	Endpoint          string `yaml:"keyspaces_endpoint"`
	Version           string `yaml:"version"`
	RegionName        string `yaml:"aws_region_name"`
	AccessKeyID       string `yaml:"aws_access_key_id"`
	SecretAccessKey   string `yaml:"aws_secret_access_key"`
	UseRoleARN        bool   `yaml:"use_role_arn"`
	AWSRoleArn        string `yaml:"aws_role_arn"`
	UseServiceAccount bool   `yaml:"use_service_account"`
	ServiceAccount    string `yaml:"service_account"`
	CreateIfNotExists bool   `yaml:"create_if_not_exists"`
}

func SchemaToKeyspacesIntegrationConfiguration(d *schema.ResourceData) KeyspacesIntegrationConfiguration {
	return KeyspacesIntegrationConfiguration{
		Name: d.Get("name").(string), Keyspace: d.Get("keyspace").(string),
		Endpoint: d.Get("keyspaces_endpoint").(string), Version: d.Get("version").(string),
		RegionName:      d.Get("aws_region_name").(string),
		AccessKeyID:     d.Get("aws_access_key_id").(string),
		SecretAccessKey: d.Get("aws_secret_access_key").(string),
		UseRoleARN:      d.Get("use_role_arn").(bool), AWSRoleArn: d.Get("aws_role_arn").(string),
		UseServiceAccount: d.Get("use_service_account").(bool),
		ServiceAccount:    d.Get("service_account").(string),
		CreateIfNotExists: d.Get("create_if_not_exists").(bool),
	}
}
