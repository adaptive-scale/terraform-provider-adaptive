package integrations

/*
Example resource usage:

resource "adaptive_msteams_workflow" "example" {
	  name = "test-wf"
	  webhook_url = "https://example.com/webhook"
}
*/

import (
	"context"
	"fmt"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

type MSTeamsWorkflowIntegrationConfiguration struct {
	Name       string `yaml:"name"`
	WebhookURL string `yaml:"webhookURL"`
}

func ResourceAdaptiveMSTeamsWorkflow() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveMSTeamsWorkflowCreate,
		ReadContext:   resourceAdaptiveMSTeamsWorkflowRead,
		UpdateContext: resourceAdaptiveMSTeamsWorkflowUpdate,
		DeleteContext: resourceAdaptiveMSTeamsWorkflowDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the MS Teams workflow integration.",
			},
			"webhook_url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The webhook URL for the MS Teams workflow.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func SchemaToMSTeamsWorkflowIntegrationConfiguration(d *schema.ResourceData) MSTeamsWorkflowIntegrationConfiguration {
	return MSTeamsWorkflowIntegrationConfiguration{
		Name:       d.Get("name").(string),
		WebhookURL: d.Get("webhook_url").(string),
	}
}

func resourceAdaptiveMSTeamsWorkflowCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToMSTeamsWorkflowIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "msteams_workflow", config, []string{}, "")
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveMSTeamsWorkflowRead(ctx, d, m)
	return nil
}

func resourceAdaptiveMSTeamsWorkflowRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveMSTeamsWorkflowUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToMSTeamsWorkflowIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	_, err = client.UpdateResource(ctx, resourceID, "msteams_workflow", config, []string{}, "")
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveMSTeamsWorkflowRead(ctx, d, m)
}

func resourceAdaptiveMSTeamsWorkflowDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
