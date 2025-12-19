package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type RabbitMQIntegrationConfiguration struct {
	Url      string `yaml:"url"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func schemaToRabbitMQIntegrationConfiguration(d *schema.ResourceData) RabbitMQIntegrationConfiguration {
	return RabbitMQIntegrationConfiguration{
		Url:      d.Get("uri").(string),
		Name:     d.Get("name").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
	}
}
