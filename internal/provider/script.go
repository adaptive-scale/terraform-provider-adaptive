package provider

import (
	"context"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAdaptiveScript() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveScriptCreate,
		ReadContext:   resourceAdaptiveScriptRead,
		UpdateContext: resourceAdaptiveScriptUpdate,
		DeleteContext: resourceAdaptiveScriptDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			},
			"command": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			},
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			}}}
}

func resourceAdaptiveScriptCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	sName, err := attrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}
	sCommand, err := attrFromSchema[string](d, "command", true)
	if err != nil {
		return diag.FromErr(err)
	}
	sEndpoint, err := attrFromSchema[string](d, "endpoint", true)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateScript(ctx, *sName, *sCommand, *sEndpoint)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	return nil
}
func resourceAdaptiveScriptRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}
func resourceAdaptiveScriptUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	scriptID := d.Id()

	if d.HasChange("endpoint") {
		return diag.Errorf("endpoint cannot be updated for a existing script")
	}

	name, err := attrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}
	command, err := attrFromSchema[string](d, "command", true)
	if err != nil {
		return diag.FromErr(err)
	}
	endpoint, err := attrFromSchema[string](d, "endpoint", true)
	if err != nil {
		return diag.FromErr(err)
	}

	if _, err := client.UpdateScript(ctx, &scriptID, name, command, endpoint); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
func resourceAdaptiveScriptDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	scriptID := d.Id()
	client := m.(*adaptive.Client)

	name, err := attrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	if _, err := client.DeleteScript(ctx, scriptID, *name); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
