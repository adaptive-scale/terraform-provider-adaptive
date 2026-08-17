package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type HerokuIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Machine  string `yaml:"machine"`
	Username string `yaml:"username"`
	Token    string `yaml:"token"`
}

func SchemaToHerokuIntegrationConfiguration(d *schema.ResourceData) HerokuIntegrationConfiguration {
	return HerokuIntegrationConfiguration{
		Name:     attrString(d, "name"),
		Machine:  attrString(d, "machine"),
		Username: attrString(d, "username"),
		Token:    attrString(d, "token"),
	}
}
