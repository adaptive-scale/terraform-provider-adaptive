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
	// Authorization model has changed since v0.4.0
	//Resource    string `json:"resource"`
	ResourceType string `json:"resource_type"`
	Permission   string `json:"permission"`
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
			"resource_type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Resource type to grant permission on. Eg. kubernetes, postgres, mysql, mongodb",
			},
			"permissions": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The permission to grant or revoke on the specified resource.",
			},
		},
	}
}

func schemaToAuthorizationConfiguration(d *schema.ResourceData) AuthorizationConfiguration {
	return AuthorizationConfiguration{
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		ResourceType: d.Get("resource_type").(string),
		Permission:   d.Get("permissions").(string),
	}
}

func resourceAdaptiveAuthorizationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToAuthorizationConfiguration(d)
	// config, err := json.Marshal(obj)
	// if err != nil {
	// 	return diag.FromErr(fmt.Errorf("could not marshal resource configuration %w", err))
	// }

	resp, err := client.CreateAuthorization(ctx, obj.Name, obj.Description, obj.Permission, obj.ResourceType)
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

	//if d.HasChange("permissions") {
	//	return diag.Errorf("Cannot change permission of authorization after published")
	//}
	//if d.HasChange("resource_type") {
	//	return diag.Errorf("Cannot change resource of authorization after published")
	//}

	obj := schemaToAuthorizationConfiguration(d)

	_, err := client.UpdateAuthorization(ctx, authID, obj.Name, obj.Description, obj.Permission, obj.ResourceType)
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
