package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type SlackWebhookIntegrationConfiguration struct {
	Version    string `yaml:"version"`
	Name       string `yaml:"name"`
	WebhookURL string `yaml:"webhookURL"`
	APIToken   string `yaml:"apiToken"`
}

func SchemaToSlackWebhookIntegrationConfiguration(d *schema.ResourceData) SlackWebhookIntegrationConfiguration {
	return SlackWebhookIntegrationConfiguration{
		Version:    "1.0",
		Name:       attrString(d, "name"),
		WebhookURL: attrString(d, "webhook_url"),
		APIToken:   attrString(d, "api_token"),
	}
}
