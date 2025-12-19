package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type CoralogixIntegrationConfiguration struct {
	Name            string `yaml:"name"`
	Url             string `yaml:"url"`
	PrivateKey      string `yaml:"privateKey"`
	ApplicationName string `yaml:"applicationName"`
	SubSystemName   string `yaml:"subSystemName"`
}

func schemaToCoralogixIntegrationConfiguration(d *schema.ResourceData) CoralogixIntegrationConfiguration {
	return CoralogixIntegrationConfiguration{
		Name:            d.Get("name").(string),
		Url:             d.Get("uri").(string),
		PrivateKey:      d.Get("private_key").(string),
		ApplicationName: d.Get("application_name").(string),
		SubSystemName:   d.Get("sub_system_name").(string),
	}
}
