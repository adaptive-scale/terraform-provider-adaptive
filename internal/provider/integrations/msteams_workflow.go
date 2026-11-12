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
	"errors"
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
		Description:   "Manages a Microsoft Teams workflow integration. Webhook credentials remain in Terraform state when the server redacts them during refresh.",
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
	return nil
}

func resourceAdaptiveMSTeamsWorkflowRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resp, err := m.(*adaptive.Client).ReadResource(ctx, d.Id(), false)
	if errors.Is(err, adaptive.ErrNotFound) {
		d.SetId("")
		return nil
	}
	if errors.Is(err, adaptive.ErrReadUnsupported) {
		return diag.Diagnostics{{Severity: diag.Warning, Summary: "Server does not support workflow refresh", Detail: "Existing Terraform state has been preserved."}}
	}
	if err != nil {
		return diag.FromErr(err)
	}
	data := resp.(map[string]interface{})
	if name, ok := adaptive.ResponseValue(data, "name", "Name"); ok {
		if err := d.Set("name", name); err != nil {
			return diag.FromErr(err)
		}
	}
	// Webhook URLs carry credentials and are withheld by the v0.9 read API.
	// Preserve the configured URL in state, as with other resource secrets.
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
	return nil
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
