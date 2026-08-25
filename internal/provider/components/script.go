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

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the script.",
			},
			"command": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				Description: "The command the script runs. Write-only: the Adaptive API never returns script " +
					"bodies, so this value cannot be read back and an import leaves it empty.",
			},
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The endpoint the script is attached to. Cannot be changed after creation.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "An optional description of the script.",
			},
			"parameter_descriptions": {
				Type:        schema.TypeMap,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "Descriptions for the script's parameters, keyed by parameter name.",
			},
			"command_digest": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Opaque server fingerprint of the script body, used to detect out-of-band " +
					"changes to the command on refresh.",
			},
		}}
}

// scriptRequestFromSchema builds the API request from resource state.
func scriptRequestFromSchema(d *schema.ResourceData) adaptive.ScriptRequest {
	return adaptive.ScriptRequest{
		Name:                  d.Get("name").(string),
		Command:               d.Get("command").(string),
		Endpoint:              d.Get("endpoint").(string),
		Description:           d.Get("description").(string),
		ParameterDescriptions: stateStringMap(d, "parameter_descriptions"),
	}
}

// refreshCommandDigest records the server's fingerprint of the script body
// after a write. Best-effort: servers that predate the field simply leave the
// digest empty, which disables drift detection rather than failing the apply.
func refreshCommandDigest(ctx context.Context, client *adaptive.Client, d *schema.ResourceData) {
	r, err := client.ReadScript(ctx, d.Id())
	if err != nil || r == nil {
		return
	}
	_ = d.Set("command_digest", r.CommandDigest)
}

func ResourceAdaptiveScriptCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	for _, attr := range []string{"name", "command", "endpoint"} {
		if _, err := integrations.AttrFromSchema[string](d, attr, true); err != nil {
			return diag.FromErr(err)
		}
	}

	resp, err := client.CreateScript(ctx, scriptRequestFromSchema(d))
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	refreshCommandDigest(ctx, client, d)
	return nil
}

func ResourceAdaptiveScriptRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	r, err := client.ReadScript(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if r == nil {
		return notFoundGone(d)
	}
	isImport := isImportRead(d, "name")
	if missingOnImport(d, isImport, r.Name) {
		return nil
	}

	if err := d.Set("name", r.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("endpoint", r.Endpoint); err != nil {
		return diag.FromErr(err)
	}
	// An optional description the configuration never set is not adopted from
	// the server, but one that was set is refreshed — including a removal.
	if prior := d.Get("description").(string); isImport || prior != "" {
		if err := d.Set("description", optString(prior, r.Description, isImport)); err != nil {
			return diag.FromErr(err)
		}
	}
	prior := stateStringMap(d, "parameter_descriptions")
	if (isImport || len(prior) > 0) && !sameStringMap(prior, r.ParameterDescriptions) {
		if err := d.Set("parameter_descriptions", r.ParameterDescriptions); err != nil {
			return diag.FromErr(err)
		}
	}

	// The script body is write-only, but the server returns an opaque
	// fingerprint of it: a mismatch with the recorded one means the command was
	// changed out-of-band. Clearing the command makes the next plan show it
	// being re-applied from the configuration.
	priorDigest := d.Get("command_digest").(string)
	if !isImport && priorDigest != "" && r.CommandDigest != "" && priorDigest != r.CommandDigest {
		if err := d.Set("command", ""); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := d.Set("command_digest", r.CommandDigest); err != nil {
		return diag.FromErr(err)
	}

	if isImport {
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "script command could not be imported",
			Detail: "the Adaptive API does not return script bodies; set command in your configuration " +
				"before the next apply (that apply will rewrite the script body)",
		}}
	}
	return nil
}

func ResourceAdaptiveScriptUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	scriptID := d.Id()

	if d.HasChange("endpoint") {
		return diag.Errorf("endpoint cannot be updated for a existing script")
	}

	for _, attr := range []string{"name", "command", "endpoint"} {
		if _, err := integrations.AttrFromSchema[string](d, attr, true); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := client.UpdateScript(ctx, scriptID, scriptRequestFromSchema(d)); err != nil {
		return diag.FromErr(err)
	}

	refreshCommandDigest(ctx, client, d)
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
