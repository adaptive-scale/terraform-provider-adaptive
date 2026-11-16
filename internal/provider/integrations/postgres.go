package integrations

/*
Example resource usage:

resource "adaptive_postgres" "example" {
  name          = "mydatabase256789"
  host          = "myhost.example.com"
  port          = "5433"
  username      = "myuser"
  password      = "mypasswor2"
  database_name = ""
}

*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type PostgresIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
	TLSRootCert  string `yaml:"rootCert"`
	TLSCertFile  string `yaml:"crtText"`
	TLSKeyFile   string `yaml:"keyText"`

	// Pointer switches preserve absent versus explicit false. Legacy authMode
	// remains authoritative only while useRdsIam is absent.
	UseRDSIAM   *bool  `yaml:"useRdsIam,omitempty"`
	AuthMode    string `yaml:"authMode,omitempty"`
	IAMIdentity `yaml:",inline"`
}

// rdsIAMFromSchema selects the same switch-over-authMode precedence as the
// backend. Leave omitted IRSA switches nil instead of inventing a default; the
// backend owns identity inference. Nonsecret identity persists while inactive,
// but conditional static credentials must survive the held-secret write guard.
func rdsIAMFromSchema(d *schema.ResourceData) PostgresIntegrationConfiguration {
	switchValue := optionalBool(d, "use_rds_iam")
	authMode := attrString(d, "auth_mode")
	active := authMode == "aws_rds_iam"
	if switchValue != nil {
		active = *switchValue
	}
	return PostgresIntegrationConfiguration{UseRDSIAM: switchValue, AuthMode: authMode, IAMIdentity: iamIdentity(d, active)}
}

// TODO: .(string) is assumption will cause problems
func SchemaToPostgresIntegrationConfiguration(d *schema.ResourceData) PostgresIntegrationConfiguration {
	sslMode := ""
	if d.Get("ssl_mode") != nil {
		if _sslMode, ok := d.Get("ssl_mode").(string); ok {
			sslMode = _sslMode
		} else if !ok {
			sslMode = ""
		}
	}

	tlsRootCert := ""
	if d.Get("tls_root_cert") != nil {
		if _tlsRootCert, ok := d.Get("tls_root_cert").(string); ok {
			tlsRootCert = _tlsRootCert
		}
	}

	tlsCertFile := ""
	if d.Get("tls_cert_file") != nil {
		if _tlsCertFile, ok := d.Get("tls_cert_file").(string); ok {
			tlsCertFile = _tlsCertFile
		}
	}

	tlsKeyFile := ""
	if d.Get("tls_key_file") != nil {
		if _tlsKeyFile, ok := d.Get("tls_key_file").(string); ok {
			tlsKeyFile = _tlsKeyFile
		}
	}

	cfg := rdsIAMFromSchema(d)
	cfg.Name = d.Get("name").(string)
	cfg.Username = d.Get("username").(string)
	cfg.Password = d.Get("password").(string)
	cfg.DatabaseName = d.Get("database_name").(string)
	cfg.HostName = d.Get("host").(string)
	cfg.Port = d.Get("port").(string)
	cfg.SSLMode = sslMode
	cfg.TLSRootCert = tlsRootCert
	cfg.TLSCertFile = tlsCertFile
	cfg.TLSKeyFile = tlsKeyFile
	return cfg
}
