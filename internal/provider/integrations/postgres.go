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

	// AWS RDS IAM authentication. These are only emitted when use_rds_iam is
	// set, so a password resource marshals exactly as it did before.
	UseRDSIAM          *bool  `yaml:"useRdsIam,omitempty"`
	UseIRSA            *bool  `yaml:"useIrsa,omitempty"`
	AWSRegion          string `yaml:"awsRegion,omitempty"`
	AWSRoleARN         string `yaml:"awsRoleArn,omitempty"`
	AWSAccessKeyID     string `yaml:"awsAccessKeyId,omitempty"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey,omitempty"`
	AWSServiceAccount  string `yaml:"awsServiceAccount,omitempty"`
}

// rdsIAMFromSchema reads the AWS RDS IAM block off a resource. It returns a
// zero value unless use_rds_iam is set, so password resources are untouched.
//
// use_irsa defaults to "no static keys were given", which matches how the
// platform infers the credential source, and an explicit value overrides it.
func rdsIAMFromSchema(d *schema.ResourceData) PostgresIntegrationConfiguration {
	useRDSIAM, _ := d.Get("use_rds_iam").(bool)
	if !useRDSIAM {
		return PostgresIntegrationConfiguration{}
	}

	accessKeyID, _ := d.Get("aws_access_key_id").(string)
	useIRSA := accessKeyID == ""
	if v, ok := d.GetOkExists("use_irsa"); ok {
		if b, ok := v.(bool); ok {
			useIRSA = b
		}
	}

	cfg := PostgresIntegrationConfiguration{
		UseRDSIAM:      &useRDSIAM,
		UseIRSA:        &useIRSA,
		AWSAccessKeyID: accessKeyID,
	}
	cfg.AWSRegion, _ = d.Get("aws_region").(string)
	cfg.AWSRoleARN, _ = d.Get("aws_role_arn").(string)
	cfg.AWSSecretAccessKey, _ = d.Get("aws_secret_access_key").(string)
	cfg.AWSServiceAccount, _ = d.Get("aws_service_account").(string)
	return cfg
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
