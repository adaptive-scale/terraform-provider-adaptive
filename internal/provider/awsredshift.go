package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSRedshiftIntegrationConfiguration struct {
	Version      string `yaml:"version"`
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
}

func schemaToAWSRedshiftIntegrationConfiguration(d *schema.ResourceData) AWSRedshiftIntegrationConfiguration {
	return AWSRedshiftIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		// SSLMode:      d.Get("ssl_mode").(string),
	}
}
