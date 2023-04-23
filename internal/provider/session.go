package provider

import (
	"context"

	adaptive "github.com/adaptive-scale/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	SessionTTLOptionNone = ""
	// hours
	SessionTTLOption3Hours = "3h"
	SessionTTLOption6Hours = "6h"
	// days
	SessionTTLOption1days   = "1d"
	SessionTTLOption3days   = "3d"
	SessionTTLOption7days   = "7d"
	SessionTTLOption30days  = "30d"
	SessionTTLOption60days  = "60d"
	SessionTTLOption180days = "180d"
	SessionTTLOption360days = "360d"
)

const (
	SessionTypeDefault = "cli"
	SessionTypeCLI     = "cli"
	SessionTypeClient  = "client"
	// TODO: Support scripts too?
)

func resourceAdaptiveSession() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveSessionCreate,
		ReadContext:   resourceAdaptiveSessionRead,
		UpdateContext: resourceAdaptiveSessionUpdate,
		DeleteContext: resourceAdaptiveSessionDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Postgres database to create.",
			},
			"resource": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The resource used to create the session.",
			},
			"session_type": {
				Type:        schema.TypeString,
				Default:     SessionTypeDefault,
				Optional:    true,
				Description: "The type of session to create.",
			},
			"ttl": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The port number of the Postgres instance to connect to.",
			},
			"authorization": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The authorization to use when creating the session.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Default:     "default",
				Optional:    true,
				Description: "The cluster in which this session should be created. If not provided will be set to default cluster set in workspace settings	of user's workspace",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceAdaptiveSessionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	sName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateSession(
		ctx,
		sName,
		d.Get("resource").(string),
		d.Get("authorization").(string),
		d.Get("cluster").(string),
		d.Get("ttl").(string),
		d.Get("session_type").(string),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveSessionRead(ctx, d, m)
	return nil
}

func resourceAdaptiveSessionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}

func resourceAdaptiveSessionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	sessionID := d.Id()

	if d.HasChange("session_type") {
		return diag.Errorf("Cannot change session_type after creation")
	}

	resp, err := client.UpdateSession(
		sessionID,
		d.Get("name").(string),
		d.Get("resource").(string),
		d.Get("authorization").(string),
		d.Get("cluster").(string),
		d.Get("ttl").(string),
		d.Get("session_type").(string),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveSessionRead(ctx, d, m)
	return nil
}

func resourceAdaptiveSessionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sessionID := d.Id()
	client := m.(*adaptive.Client)

	_, err := client.DeleteSession(sessionID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
