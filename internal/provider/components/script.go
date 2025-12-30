package components

import (
	"context"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveScript() *schema.Resource {
	return &schema.Resource{
		CreateContext: ResourceAdaptiveScriptCreate,
		ReadContext:   ResourceAdaptiveScriptRead,
		UpdateContext: ResourceAdaptiveScriptUpdate,
		DeleteContext: ResourceAdaptiveScriptDelete,

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

func ResourceAdaptiveScriptCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	sName, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}
	sCommand, err := integrations.AttrFromSchema[string](d, "command", true)
	if err != nil {
		return diag.FromErr(err)
	}
	sEndpoint, err := integrations.AttrFromSchema[string](d, "endpoint", true)
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
func ResourceAdaptiveScriptRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}
func ResourceAdaptiveScriptUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	scriptID := d.Id()

	if d.HasChange("endpoint") {
		return diag.Errorf("endpoint cannot be updated for a existing script")
	}

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}
	command, err := integrations.AttrFromSchema[string](d, "command", true)
	if err != nil {
		return diag.FromErr(err)
	}
	endpoint, err := integrations.AttrFromSchema[string](d, "endpoint", true)
	if err != nil {
		return diag.FromErr(err)
	}

	if _, err := client.UpdateScript(ctx, &scriptID, name, command, endpoint); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
func ResourceAdaptiveScriptDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	scriptID := d.Id()
	client := m.(*adaptive.Client)

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	if _, err := client.DeleteScript(ctx, scriptID, *name); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
