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

func resourceAdaptiveMongoAtlas() *schema.Resource {
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

type MongoAtlasIntegrationConfiguration struct {
	Name           string `yaml:"name"`
	OrganisationID string `yaml:"organization_id"`
	PublicKey      string `yaml:"public_key"`
	PrivateKey     string `yaml:"private_key"`
	ProjectID      string `yaml:"project_id"`
	URI            string `yaml:"uri"`
}

// TODO: .(string) is assumption will cause problems
func schemaToMongoAtlasIntegrationConfiguration(d *schema.ResourceData) MongoAtlasIntegrationConfiguration {
	return MongoAtlasIntegrationConfiguration{
		Name:           d.Get("name").(string),
		URI:            d.Get("uri").(string),
		OrganisationID: d.Get("organization_id").(string),
		PublicKey:      d.Get("public_key").(string),
		PrivateKey:     d.Get("private_key").(string),
	}
}

func resourceAdaptiveMongoAtlasCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToMongoIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "mongodb_atlas", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveMongoRead(ctx, d, m)
	return nil
}

func resourceAdaptiveMongoAtlasRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveMongoAtlasUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToMongoIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "mongodb", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveMongoRead(ctx, d, m)
}

func resourceAdaptiveMongoAtlasDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
