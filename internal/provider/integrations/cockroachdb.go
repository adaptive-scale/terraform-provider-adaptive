// cockroachdb.go
package integrations

/*
Example resource usage:

	resource "adaptive_cockraochdb" "example" {
		name          = "mydatabase256789"
		host          = "myhost.example.com"
		port          = "5433"
		username      = "myuser"
		password      = "mypasswor2"
		database_name = ""
		root_cert	 =  ""
	}
*/

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type CockroachDBIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
	RootCert     string `yaml:"rootCert"`
}

func SchemaToCockroachDBIntegrationConfiguration(d *schema.ResourceData) CockroachDBIntegrationConfiguration {
	sslMode := ""
	if d.Get("ssl_mode") != nil {
		if _sslMode, ok := d.Get("ssl_mode").(string); ok {
			sslMode = _sslMode
		} else if !ok {
			sslMode = ""
		}
	}

	tlsRootCert := ""
	if d.Get("root_cert") != nil {
		if _tlsRootCert, ok := d.Get("tls_root_cert").(string); ok {
			tlsRootCert = _tlsRootCert
		}
	}

	return CockroachDBIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      sslMode,
		RootCert:     strings.TrimSpace(tlsRootCert),
	}
}
