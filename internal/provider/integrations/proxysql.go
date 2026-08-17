package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ProxySQLIntegrationConfiguration struct {
	Version               string `yaml:"version"`
	Name                  string `yaml:"name"`
	Username              string `yaml:"username"`
	Password              string `yaml:"password"`
	DatabaseName          string `yaml:"databaseName"`
	HostName              string `yaml:"hostname"`
	Port                  string `yaml:"port"`
	SSLMode               string `yaml:"sslMode"`
	RootCert              string `yaml:"rootCert"`
	ClientCert            string `yaml:"clientCert"`
	ClientKey             string `yaml:"clientKey"`
	OldVersion            bool   `yaml:"oldVersion"`
	ProxysqlAdminPort     string `yaml:"proxysqlAdminPort"`
	ProxysqlAdminUsername string `yaml:"proxysqlAdminUsername"`
	ProxysqlAdminPassword string `yaml:"proxysqlAdminPassword"`
	ProxysqlHostgroupID   string `yaml:"proxysqlHostgroupID"`
}

func SchemaToProxySQLIntegrationConfiguration(d *schema.ResourceData) ProxySQLIntegrationConfiguration {
	return ProxySQLIntegrationConfiguration{
		Version:               "1.0",
		Name:                  attrString(d, "name"),
		Username:              attrString(d, "username"),
		Password:              attrString(d, "password"),
		DatabaseName:          attrString(d, "database_name"),
		HostName:              attrString(d, "host"),
		Port:                  attrString(d, "port"),
		SSLMode:               attrString(d, "ssl_mode"),
		RootCert:              strings.TrimSpace(attrString(d, "root_cert")),
		ClientCert:            strings.TrimSpace(attrString(d, "client_cert")),
		ClientKey:             strings.TrimSpace(attrString(d, "client_key")),
		OldVersion:            attrBool(d, "old_version"),
		ProxysqlAdminPort:     attrString(d, "proxysql_admin_port"),
		ProxysqlAdminUsername: attrString(d, "proxysql_admin_username"),
		ProxysqlAdminPassword: attrString(d, "proxysql_admin_password"),
		ProxysqlHostgroupID:   attrString(d, "proxysql_hostgroup_id"),
	}
}
