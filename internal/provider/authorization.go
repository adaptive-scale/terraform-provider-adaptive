// authorization.go
package provider

/*
resource "adaptive_authorization" "example" {
	name = "instance-name"
	description = ""
	resource = ""
	permission - ""
*/

import (
	"context"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AuthorizationConfiguration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Permission  string `json:"permission"`
}

func resourceAdaptiveAuthorization() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveAuthorizationCreate,
		ReadContext:   resourceAdaptiveAuthorizationRead,
		UpdateContext: resourceAdaptiveAuthorizationUpdate,
		DeleteContext: resourceAdaptiveAuthorizationDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the authorization object.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "An optional description of the authorization object.",
			},
			"resource": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the resource to apply the permission to.",
			},
			"permission": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The permission to grant or revoke on the specified resource.",
			},
		},
	}
}

func schemaToAuthorizationConfiguration(d *schema.ResourceData) AuthorizationConfiguration {
	return AuthorizationConfiguration{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Resource:    d.Get("resource").(string),
		Permission:  d.Get("permission").(string),
	}
}

func resourceAdaptiveAuthorizationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToAuthorizationConfiguration(d)
	// config, err := json.Marshal(obj)
	// if err != nil {
	// 	return diag.FromErr(fmt.Errorf("could not marshal resource configuration %w", err))
	// }

	resp, err := client.CreateAuthorization(ctx, obj.Name, obj.Description, obj.Permission, obj.Resource)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	// resourceAdaptiveAuthorizationRead(ctx, d, m)
	return nil
}

func resourceAdaptiveAuthorizationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveAuthorizationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	authID := d.Id()

	if d.HasChange("permission") {
		return diag.Errorf("Cannot change permission of authorization after published")
	}
	if d.HasChange("resource") {
		return diag.Errorf("Cannot change resource of authorization after published")
	}

	_, err := client.UpdateAuthorization(ctx, authID, d.Get("name").(string), d.Get("description").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	// resourceAdaptiveAuthorizationRead(ctx, d, m)
	return nil
}

func resourceAdaptiveAuthorizationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteAuthorization(ctx, resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
