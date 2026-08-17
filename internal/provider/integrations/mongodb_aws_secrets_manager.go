package integrations

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MongoDBAWSIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
	Key      string `yaml:"key"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMongoAWSIntegrationConfiguration(d *schema.ResourceData) MongoDBAWSIntegrationConfiguration {
	return MongoDBAWSIntegrationConfiguration{
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
		Key:      d.Get("key").(string),
	}
}
