package provider

/*
resource "adaptive_azure" "azure1" {
	name = "thing1"
	tenant_id = ""
	application_id = ""
	client_secret = ""
}
*/
import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AzureIntegrationConfiguration struct {
	Version       string `yaml:"version"`
	Name          string `yaml:"name"`
	TenantID      string `yaml:"tenantID"`
	ApplicationID string `yaml:"applicationID"`
	ClientSecret  string `yaml:"clientSecret"`
}

// schemaToAzureIntegrationConfiguration converts the Terraform schema to an
// AzureIntegrationConfiguration struct that can be serialized as YAML for
// making API calls to the Adaptive Scale platform.
func schemaToAzureIntegrationConfiguration(d *schema.ResourceData) AzureIntegrationConfiguration {
	return AzureIntegrationConfiguration{
		Version:       "1.0",
		Name:          d.Get("name").(string),
		TenantID:      d.Get("tenant_id").(string),
		ApplicationID: d.Get("application_id").(string),
		ClientSecret:  d.Get("client_secret").(string),
	}
}

// func resourceAdaptiveAzure() *schema.Resource {
// 	return &schema.Resource{
// 		CreateContext: resourceAdaptiveAzureCreate,
// 		ReadContext:   resourceAdaptiveAzureRead,
// 		UpdateContext: resourceAdaptiveAzureUpdate,
// 		DeleteContext: resourceAdaptiveAzureDelete,

// 		Schema: map[string]*schema.Schema{
// 			"name": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The name of the Azure integration to create.",
// 			},
// 			"tenant_id": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The Azure tenant ID.",
// 			},
// 			"application_id": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The Azure application ID.",
// 			},
// 			"client_secret": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The Azure client secret.",
// 			},
// 			"last_updated": {
// 				Type:     schema.TypeString,
// 				Optional: true,
// 				Computed: true,
// 			},
// 		},
// 	}
// }

// func resourceAdaptiveAzureCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)

// 	obj := schemaToAzureIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	rName, err := nameFromSchema(d)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}
// 	resp, err := client.CreateResource(ctx, rName, "azure", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId(resp.ID)
// 	resourceAdaptiveAzureRead(ctx, d, m)
// 	return nil
// }

// func resourceAdaptiveAzureRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	return nil
// }

// func resourceAdaptiveAzureUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)
// 	resourceID := d.Id()

// 	obj := schemaToAzureIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	_, err = client.UpdateResource(resourceID, "azure", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.Set("last_updated", time.Now())
// 	return resourceAdaptiveAzureRead(ctx, d, m)
// }

// func resourceAdaptiveAzureDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	resourceID := d.Id()
// 	client := m.(*adaptive.Client)
// 	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId("")
// 	return nil
// }
