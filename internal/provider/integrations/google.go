package integrations

/*
resource "adaptive_google" "example" {
	name = "instance-name"
	domain = ""
	client_id = ""
	client_secret = ""
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

func resourceAdaptiveGoogle() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveGoogleCreate,
		ReadContext:   resourceAdaptiveGoogleRead,
		UpdateContext: resourceAdaptiveGoogleUpdate,
		DeleteContext: resourceAdaptiveGoogleDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Google OAuth integration to create.",
			},
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "https://accounts.google.com",
				Description: "A domain to restrict the Google OAuth integration to.",
			},
			"client_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The client ID for the Google OAuth integration.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The client secret for the Google OAuth integration.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

type GoogleOAuthIntegrationConfiguration struct {
	Version      string `yaml:"Version"`
	Name         string `yaml:"name"`
	Domain       string `yaml:"domain"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

func SchemaToGoogleOAuthIntegrationConfiguration(d *schema.ResourceData) GoogleOAuthIntegrationConfiguration {
	return GoogleOAuthIntegrationConfiguration{
		Version:      "1",
		Name:         d.Get("name").(string),
		Domain:       d.Get("domain").(string),
		ClientID:     d.Get("client_id").(string),
		ClientSecret: d.Get("client_secret").(string),
	}
}

func resourceAdaptiveGoogleCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToGoogleOAuthIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateResource(ctx, rName, "google", config, []string{}, "")
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveGoogleRead(ctx, d, m)
	return nil
}

func resourceAdaptiveGoogleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveGoogleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToGoogleOAuthIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	_, err = client.UpdateResource(ctx, resourceID, "google", config, []string{}, "")
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveGoogleRead(ctx, d, m)
}

func resourceAdaptiveGoogleDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
