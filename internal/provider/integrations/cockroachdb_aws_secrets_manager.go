package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// CockroachDBSecretsManagerIntegrationConfiguration carries the db_* overrides
// the integration reads in addition to the four fields the secret itself
// supplies; they stay out of the blob unless set.
type CockroachDBSecretsManagerIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	ARN          string `yaml:"arn"`
	Region       string `yaml:"region"`
	SecretID     string `yaml:"secret_id"`
	Username     string `yaml:"db_user,omitempty"`
	Password     string `yaml:"db_password,omitempty"`
	DatabaseName string `yaml:"db_name,omitempty"`
	HostName     string `yaml:"db_host,omitempty"`
	Port         string `yaml:"db_port,omitempty"`
	RootCert     string `yaml:"db_rootCert,omitempty"`
	SSLMode      string `yaml:"sslMode,omitempty"`
}

func SchemaToCockroachDBSecretsManagerIntegrationConfiguration(d *schema.ResourceData) CockroachDBSecretsManagerIntegrationConfiguration {
	return CockroachDBSecretsManagerIntegrationConfiguration{
		Name:         attrString(d, "name"),
		ARN:          attrString(d, "arn"),
		Region:       attrString(d, "region"),
		SecretID:     attrString(d, "secret_id"),
		Username:     attrString(d, "username"),
		Password:     attrString(d, "password"),
		DatabaseName: attrString(d, "database_name"),
		HostName:     attrString(d, "host"),
		Port:         attrString(d, "port"),
		RootCert:     strings.TrimSpace(attrString(d, "root_cert")),
		SSLMode:      attrString(d, "ssl_mode"),
	}
}
