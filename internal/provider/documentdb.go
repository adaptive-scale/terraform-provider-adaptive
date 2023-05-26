package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DocumentDBIntegrationConfiguration struct {
	Name          string `yaml:"name"`
	URI           string `yaml:"uri"`
	URISecretPath string `yaml:"uriSecretPath"`
}

// TODO: .(string) is assumption will cause problems
func schemaToDocumentDBIntegrationConfiguration(d *schema.ResourceData) DocumentDBIntegrationConfiguration {
	return DocumentDBIntegrationConfiguration{
		Name:          d.Get("name").(string),
		URI:           d.Get("uri").(string),
		URISecretPath: d.Get("uri_secret_path").(string),
	}
}

// func resourceAdaptiveMongo() *schema.Resource {
// 	return &schema.Resource{
// 		CreateContext: resourceAdaptiveMongoCreate,
// 		ReadContext:   resourceAdaptiveMongoRead,
// 		UpdateContext: resourceAdaptiveMongoUpdate,
// 		DeleteContext: resourceAdaptiveMongoDelete,

// 		Schema: map[string]*schema.Schema{
// 			"name": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The name of the MongoDB instance to create.",
// 			},
// 			"uri": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The connection string for the MongoDB instance to connect to.",
// 			},
// 			"last_updated": {
// 				Type:     schema.TypeString,
// 				Optional: true,
// 				Computed: true,
// 			},
// 		},
// 	}
// }

// func resourceAdaptiveMongoCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)

// 	obj := schemaToMongoIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	rName, err := nameFromSchema(d)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}
// 	resp, err := client.CreateResource(ctx, rName, "mongodb", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId(resp.ID)
// 	resourceAdaptiveMongoRead(ctx, d, m)
// 	return nil
// }

// func resourceAdaptiveMongoRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	return nil
// }

// func resourceAdaptiveMongoUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)
// 	resourceID := d.Id()

// 	obj := schemaToMongoIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	_, err = client.UpdateResource(resourceID, "mongodb", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.Set("last_updated", time.Now())
// 	return resourceAdaptiveMongoRead(ctx, d, m)
// }

// func resourceAdaptiveMongoDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	resourceID := d.Id()
// 	client := m.(*adaptive.Client)
// 	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId("")
// 	return nil
// }
