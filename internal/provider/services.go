package provider

/*
Example resource usage:

resource "adaptive_servicelist" "example" {
	name          = "mydatabase256789"
	urls      = "comma,separated,urls"
}
*/

import (
	"context"
	"errors"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

type ServiceListIntegrationConfiguration struct {
	Version string `yaml:"version"`
	URLs    string `yaml:"urls"`
}

func resourceAdaptiveServiceList() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveServiceListCreate,
		ReadContext:   resourceAdaptiveServiceListRead,
		UpdateContext: resourceAdaptiveServiceListUpdate,
		DeleteContext: resourceAdaptiveServiceListDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the service list to create.",
			},
			"urls": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Comma-separated list of URLs.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func schemaToServiceListIntegrationConfiguration(d *schema.ResourceData) ServiceListIntegrationConfiguration {
	return ServiceListIntegrationConfiguration{
		Version: "1",
		URLs:    d.Get("urls").(string),
	}
}

func resourceAdaptiveServiceListCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToServiceListIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "servicelist", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveServiceListRead(ctx, d, m)
	return nil
}

func resourceAdaptiveServiceListRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveServiceListUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToServiceListIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "servicelist", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveServiceListRead(ctx, d, m)
}

func resourceAdaptiveServiceListDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
