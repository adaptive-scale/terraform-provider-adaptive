package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSElastiCacheIntegrationConfiguration struct {
	UseIAMAuth           *bool  `yaml:"useIamAuth,omitempty"`
	UseIRSA              *bool  `yaml:"useIrsa,omitempty"`
	AWSRoleARN           string `yaml:"aws_role_arn,omitempty"`
	AWSServiceAccount    string `yaml:"aws_service_account,omitempty"`
	CacheName            string `yaml:"cache_name,omitempty"`
	CacheType            string `yaml:"cache_type,omitempty"`
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
		UseIAMAuth:           optionalBool(d, "use_iam_auth"),
		UseIRSA:              optionalBool(d, "use_irsa"),
		AWSRoleARN:           attrString(d, "aws_role_arn"),
		AWSServiceAccount:    attrString(d, "aws_service_account"),
		CacheName:            attrString(d, "cache_name"),
		CacheType:            attrString(d, "cache_type"),
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
