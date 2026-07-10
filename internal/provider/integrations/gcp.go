package integrations

/*
resource "adaptive_gcp" "example" {
	name = "instance-name"
	project_id = ""
	key_file = ""
*/

import (
	"context"
	"fmt"
	"strings"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

type GCPIntegrationConfiguration struct {
	Version   string `yaml:"version"`
	Name      string `yaml:"name"`
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

func SchemaToGCPIntegrationConfiguration(d *schema.ResourceData) GCPIntegrationConfiguration {
	return GCPIntegrationConfiguration{
		Version:   "1",
		Name:      d.Get("name").(string),
		ProjectID: d.Get("project_id").(string),
		KeyFile:   strings.TrimSpace(d.Get("key_file").(string)),
	}
}

func resourceAdaptiveGCPCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToGCPIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "gcp", config, []string{}, "")
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

	obj := SchemaToGCPIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	_, err = client.UpdateResource(ctx, resourceID, "gcp", config, []string{}, "")
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveGCPRead(ctx, d, m)
}

func resourceAdaptiveGCPDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
