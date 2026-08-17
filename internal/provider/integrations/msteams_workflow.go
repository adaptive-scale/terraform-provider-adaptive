package integrations

/*
Example resource usage:

resource "adaptive_msteams_workflow" "example" {
	  name = "test-wf"
	  webhook_url = "https://example.com/webhook"
}
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MSTeamsWorkflowIntegrationConfiguration struct {
	Name       string `yaml:"name"`
	WebhookURL string `yaml:"webhookURL"`
}

func SchemaToMSTeamsWorkflowIntegrationConfiguration(d *schema.ResourceData) MSTeamsWorkflowIntegrationConfiguration {
	return MSTeamsWorkflowIntegrationConfiguration{
		Name:       d.Get("name").(string),
		WebhookURL: d.Get("webhook_url").(string),
	}
}
