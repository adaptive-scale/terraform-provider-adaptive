package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SyslogIntegrationConfiguration struct {
	TLSEnabled         bool   `yaml:"tlsEnabled"`
	CACertificate      string `yaml:"caCertificate"`
	ClientCertificate  string `yaml:"clientCertificate"`
	ClientKey          string `yaml:"clientKey"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`

	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Port     string `yaml:"port"`
	Protocol string `yaml:"protocol"`
}

func SchemaToSyslogIntegrationConfiguration(d *schema.ResourceData) SyslogIntegrationConfiguration {
	return SyslogIntegrationConfiguration{
		TLSEnabled:         attrBool(d, "tls_enabled"),
		CACertificate:      attrString(d, "tls_ca_cert"),
		ClientCertificate:  attrString(d, "client_certificate"),
		ClientKey:          attrString(d, "client_key"),
		InsecureSkipVerify: attrBool(d, "tls_skip_verify"),
		Name:               d.Get("name").(string),
		Hostname:           d.Get("hostname").(string),
		Port:               d.Get("port").(string),
		Protocol:           d.Get("protocol").(string),
	}
}
