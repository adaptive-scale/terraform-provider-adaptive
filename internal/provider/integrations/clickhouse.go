package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ClickHouseIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
}

func SchemaToClickHouseIntegrationConfiguration(d *schema.ResourceData) ClickHouseIntegrationConfiguration {
	sslMode := ""
	if v, ok := d.GetOk("ssl_mode"); ok {
		sslMode = v.(string)
	}

	return ClickHouseIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      sslMode,
	}
}
