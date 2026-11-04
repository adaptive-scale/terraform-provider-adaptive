package integrations

/*
Example resource usage:

resource "adaptive_mysql" "example" {
	  name          = "mydatabase256789"
	  database_name = ""
	  host          = "myhost.example.com"
	  port          = "5433"
	  username      = "myuser"
	  password      = "mypasswor2"
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MySQLIntegrationConfiguration struct {
	ClientCert string `yaml:"clientCert"`
	ClientKey  string `yaml:"clientKey"`
	RootCert   string `yaml:"rootCert"`
	OldVersion bool   `yaml:"oldVersion"`

	UseRDSIAM    *bool  `yaml:"useRdsIam,omitempty"`
	AuthMode     string `yaml:"authMode,omitempty"`
	IAMIdentity  `yaml:",inline"`
	Version      string `yaml:"version"`
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMySQLIntegrationConfiguration(d *schema.ResourceData) MySQLIntegrationConfiguration {
	iam := rdsIAMFromSchema(d)
	return MySQLIntegrationConfiguration{
		ClientCert:   attrString(d, "client_cert"),
		ClientKey:    attrString(d, "client_key"),
		RootCert:     attrString(d, "root_cert"),
		OldVersion:   attrBool(d, "old_version"),
		UseRDSIAM:    iam.UseRDSIAM,
		AuthMode:     iam.AuthMode,
		IAMIdentity:  iam.IAMIdentity,
		Version:      "",
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      mysqlSSLMode(d),
	}
}

// Retain the historic require default only for a genuinely absent mode. Use
// planned values, not raw configured values, so ignore_changes remains honored.
func mysqlSSLMode(d *schema.ResourceData) string {
	if mode, exists := d.GetOkExists("ssl_mode"); exists {
		if value, ok := mode.(string); ok {
			return value
		}
	}
	raw := d.GetRawConfig()
	if raw.IsKnown() && !raw.IsNull() && raw.Type().IsObjectType() && raw.Type().HasAttribute("ssl_mode") && !raw.GetAttr("ssl_mode").IsNull() {
		return attrString(d, "ssl_mode")
	}
	return "require"
}
