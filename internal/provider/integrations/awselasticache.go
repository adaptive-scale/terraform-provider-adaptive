package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSElastiCacheIntegrationConfiguration struct {
	Version              string `yaml:"version"`
	Name                 string `yaml:"name"`
	Username             string `yaml:"username"`
	Password             string `yaml:"password"`
	Host                 string `yaml:"host"`
	Port                 string `yaml:"port"`
	TlsEnabled           bool   `yaml:"tlsEnabled"`
	TlsSkipVerify        bool   `yaml:"tlsSkipVerify"`
	AccessControlMethod  string `yaml:"access_control_method"`
	AccessControlGroupID string `yaml:"access_control_group"`
	AWSRegionName        string `yaml:"aws_region_name"`
	AWSAccessKeyID       string `yaml:"aws_access_key_id"`
	AWSSecretAccessKey   string `yaml:"aws_secret_access_key"`
}

func SchemaToAWSElastiCacheIntegrationConfiguration(d *schema.ResourceData) AWSElastiCacheIntegrationConfiguration {
	return AWSElastiCacheIntegrationConfiguration{
		Version:              "1.0",
		Name:                 attrString(d, "name"),
		Username:             attrString(d, "username"),
		Password:             attrString(d, "password"),
		Host:                 attrString(d, "host"),
		Port:                 attrString(d, "port"),
		TlsEnabled:           attrBool(d, "tls_enabled"),
		TlsSkipVerify:        attrBool(d, "tls_skip_verify"),
		AccessControlMethod:  attrString(d, "access_control_method"),
		AccessControlGroupID: attrString(d, "access_control_group"),
		AWSRegionName:        attrString(d, "aws_region_name"),
		AWSAccessKeyID:       attrString(d, "aws_access_key_id"),
		AWSSecretAccessKey:   attrString(d, "aws_secret_access_key"),
	}
}
