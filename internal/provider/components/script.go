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
		// Import takes the object's id; Read then hydrates the rest of state.
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			},
			"command": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				// A script body routinely embeds credentials, which is also why
				// the backend withholds it from reads.
				Description: "The command the script runs. Not refreshed on read: the backend withholds script bodies because they can embed credentials.",
			},
			"endpoint": {
				Type:     schema.TypeString,
				Required: true,
				// The backend cannot move a script between endpoints. This used to
				// be enforced by erroring during Update, which surfaced only at
				// apply time; ForceNew shows it as a replacement during plan.
				ForceNew:    true,
				Description: "The endpoint the script belongs to. Changing this replaces the script.",
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
	client := m.(*adaptive.Client)

	script, err := client.GetScript(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out of band — drop it so Terraform plans a recreate.
	if script == nil {
		d.SetId("")
		return nil
	}

	if err := d.Set("name", script.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("endpoint", script.Endpoint); err != nil {
		return diag.FromErr(err)
	}
	// command is intentionally not refreshed: a script body can embed
	// credentials, so the backend withholds it (CommandOmitted). Setting it from
	// an empty response would read as "the command was cleared upstream" and
	// propose rewriting it on every plan.

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
