package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type RedisIntegrationConfiguration struct {
	Version       string `yaml:"version"`
	Name          string `yaml:"name"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	Host          string `yaml:"host"`
	Port          string `yaml:"port"`
	TlsEnabled    bool   `yaml:"tlsEnabled"`
	TlsSkipVerify bool   `yaml:"tlsSkipVerify"`
	IsRedisLabs   bool   `yaml:"isRedisLabs"`
	CertFile      string `yaml:"crtText"`
	KeyFile       string `yaml:"keyText"`
	CAFile        string `yaml:"caText"`
}

func SchemaToRedisIntegrationConfiguration(d *schema.ResourceData) RedisIntegrationConfiguration {
	return RedisIntegrationConfiguration{
		Version:       "1.0",
		Name:          attrString(d, "name"),
		Username:      attrString(d, "username"),
		Password:      attrString(d, "password"),
		Host:          attrString(d, "host"),
		Port:          attrString(d, "port"),
		TlsEnabled:    attrBool(d, "tls_enabled"),
		TlsSkipVerify: attrBool(d, "tls_skip_verify"),
		IsRedisLabs:   attrBool(d, "is_redis_labs"),
		CertFile:      strings.TrimSpace(attrString(d, "tls_cert_file")),
		KeyFile:       strings.TrimSpace(attrString(d, "tls_key_file")),
		CAFile:        strings.TrimSpace(attrString(d, "tls_ca_cert")),
	}
}
