package provider

import (
	"context"
	"errors"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func resourceAdaptiveDocumentDB() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveMongoCreate,
		ReadContext:   resourceAdaptiveMongoRead,
		UpdateContext: resourceAdaptiveMongoUpdate,
		DeleteContext: resourceAdaptiveMongoDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the MongoDB instance to create.",
			},
			"uri": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The connection string for the MongoDB instance to connect to.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

type AWSDocumentDBIntegrationConfiguration struct {
	Name string `yaml:"name"`
	URI  string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func schemaToAWSDocumentDBIntegrationConfiguration(d *schema.ResourceData) AWSDocumentDBIntegrationConfiguration {
	return AWSDocumentDBIntegrationConfiguration{
		Name: d.Get("name").(string),
		URI:  d.Get("uri").(string),
	}
}

func resourceAdaptiveAWSDocumentDBCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToAWSDocumentDBIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "AWSDocumentDBdb", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveAWSDocumentDBRead(ctx, d, m)
	return nil
}

func resourceAdaptiveAWSDocumentDBRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveAWSDocumentDBUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToAWSDocumentDBIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "awsdocumentdb", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveAWSDocumentDBRead(ctx, d, m)
}

func resourceAdaptiveAWSDocumentDBDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
