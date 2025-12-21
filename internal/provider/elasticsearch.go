package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ElasticsearchIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Index    string `yaml:"index"`
}

func schemaToElasticsearchIntegrationConfiguration(d *schema.ResourceData) ElasticsearchIntegrationConfiguration {
	return ElasticsearchIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Url:      d.Get("uri").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
		Index:    d.Get("index").(string),
	}
}
