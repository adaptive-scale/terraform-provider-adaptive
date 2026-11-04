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
		CustomizeDiff: familyPlan(familyInputPlan([]string{"name", "command", "endpoint"}, nil), rejectExistingChange("endpoint", "script endpoint bindings are immutable"), rejectExistingClear("description", "", descriptionClearBlocked)),
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
				Description: "An optional description of the script. The current backend preserves empty descriptions on update, so clearing an existing description is rejected at plan until backend support is available.",
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
	metadata := stateStringMap(d, "parameter_descriptions")
	if metadata == nil {
		metadata = map[string]string{}
	}
	return adaptive.ScriptRequest{
		Name:                  d.Get("name").(string),
		Command:               d.Get("command").(string),
		Endpoint:              d.Get("endpoint").(string),
		Description:           d.Get("description").(string),
		ParameterDescriptions: metadata,
	}
}

// refreshCommandDigest records the server's fingerprint of the script body
// after a write. Older servers or failed reads preserve prior evidence and
// warn that this write has no new acknowledged baseline, without tainting the ID.
func refreshCommandDigest(ctx context.Context, client *adaptive.Client, d *schema.ResourceData) diag.Diagnostics {
	r, err := client.ReadScript(ctx, d.Id())
	if err != nil || r == nil || r.CommandDigest == "" {
		return diag.Diagnostics{{Severity: diag.Warning, Summary: "script written; command fingerprint unavailable", Detail: "The script ID is retained. The backend read did not provide a command fingerprint; prior fingerprint evidence is preserved, but this write cannot establish a new drift baseline. Refresh after backend recovery. No command body is included in this diagnostic."}}
	}
	if err := d.Set("command_digest", r.CommandDigest); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveScriptCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "command", "endpoint"}, nil); err != nil {
		return diag.FromErr(err)
	}
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
	if resp == nil || resp.ID == "" {
		return diag.Errorf("script create returned no trackable identity; inspect the remote object before retrying")
	}
	d.SetId(resp.ID)

	return refreshCommandDigest(ctx, client, d)
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
	// Refresh never acknowledges an external write or erases missing evidence.
	if priorDigest == "" && r.CommandDigest != "" {
		if err := d.Set("command_digest", r.CommandDigest); err != nil {
			return diag.FromErr(err)
		}
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
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "command", "endpoint"}, nil); err != nil {
		return diag.FromErr(err)
	}
	if d.HasChange("description") && d.Get("description") == "" {
		return diag.Errorf("description cannot be cleared: %s", descriptionClearBlocked)
	}
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

	return refreshCommandDigest(ctx, client, d)
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
