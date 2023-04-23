package provider

/*
Example resource usage:

resource "adaptive_okta" "example" {
	name = "instance-name"
	domain = ""
	client_id = ""
	client_secret = ""
}
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

type OktaOAuthIntegrationConfiguration struct {
	Version      string `yaml:"version"`
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

func resourceAdaptiveOkta() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveOktaCreate,
		ReadContext:   resourceAdaptiveOktaRead,
		UpdateContext: resourceAdaptiveOktaUpdate,
		DeleteContext: resourceAdaptiveOktaDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Okta OAuth integration to create.",
			},
			"domain": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Okta domain to use for authentication.",
			},
			"client_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The client ID of the Okta OAuth application.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The client secret of the Okta OAuth application.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

// TODO: .(string) is assumption will cause problems
func schemaToOktaIntegrationConfiguration(d *schema.ResourceData) OktaOAuthIntegrationConfiguration {
	return OktaOAuthIntegrationConfiguration{
		Version:      "1.0",
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
	}
}

func resourceAdaptiveOktaCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToOktaIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "okta", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveOktaRead(ctx, d, m)
	return nil
}

func resourceAdaptiveOktaRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveOktaUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToOktaIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "okta", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveOktaRead(ctx, d, m)
}

func resourceAdaptiveOktaDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
