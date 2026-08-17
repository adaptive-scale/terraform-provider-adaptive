package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSDocumentDBSecretsManagerIntegrationConfiguration struct {
	Version    string `yaml:"version"`
	Name       string `yaml:"name"`
	ARN        string `yaml:"arn"`
	Region     string `yaml:"region"`
	SecretID   string `yaml:"secret_id"`
	TlsEnabled bool   `yaml:"tlsEnabled"`
}

func SchemaToAWSDocumentDBSecretsManagerIntegrationConfiguration(d *schema.ResourceData) AWSDocumentDBSecretsManagerIntegrationConfiguration {
	return AWSDocumentDBSecretsManagerIntegrationConfiguration{
		Version:    "1.0",
		Name:       attrString(d, "name"),
		ARN:        attrString(d, "arn"),
		Region:     attrString(d, "region"),
		SecretID:   attrString(d, "secret_id"),
		TlsEnabled: attrBool(d, "tls_enabled"),
	}
}
