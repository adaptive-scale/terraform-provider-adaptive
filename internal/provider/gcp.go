package provider

/*
resource "adaptive_gcp" "example" {
	name = "instance-name"
	project_id = ""
	key_file = ""
*/

import (
	"context"
	"errors"
	"time"

	adaptive "github.com/adaptive-scale/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

type GCPIntegrationConfiguration struct {
	Version   string `yaml:"version"`
	ProjectID string `yaml:"project_id"`
	KeyFile   string `yaml:"key_file"`
}

func resourceAdaptiveGCP() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveGCPCreate,
		ReadContext:   resourceAdaptiveGCPRead,
		UpdateContext: resourceAdaptiveGCPUpdate,
		DeleteContext: resourceAdaptiveGCPDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the GCP instance to create.",
			},
			"project_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The GCP project ID.",
			},
			"key_file": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The path to the GCP service account key file.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func schemaToGCPIntegrationConfiguration(d *schema.ResourceData) GCPIntegrationConfiguration {
	return GCPIntegrationConfiguration{
		Version:   "1",
		ProjectID: d.Get("project_id").(string),
		KeyFile:   d.Get("key_file").(string),
	}
}

func resourceAdaptiveGCPCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToGCPIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "gcp", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveGCPRead(ctx, d, m)
	return nil
}

func resourceAdaptiveGCPRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveGCPUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToGCPIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "gcp", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveGCPRead(ctx, d, m)
}

func resourceAdaptiveGCPDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
