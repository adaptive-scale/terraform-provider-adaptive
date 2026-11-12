package components

import (
	"context"
	"errors"

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
			"description":            {Type: schema.TypeString, Optional: true, Computed: true, Description: "Script description. Omit to retain the server value."},
			"help_text":              {Type: schema.TypeString, Optional: true, Computed: true, Description: "Help displayed before running the script. An empty string clears it."},
			"parameter_descriptions": {Type: schema.TypeMap, Optional: true, Computed: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Descriptions keyed by script parameter name."},
			"command_digest":         {Type: schema.TypeString, Computed: true, Description: "Opaque server fingerprint of the write-only command, when supported."},
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

	resp, err := client.CreateScript(ctx, *sName, *sCommand, *sEndpoint, scriptOptionsFromSchema(d))
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	return nil
}
func ResourceAdaptiveScriptRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	data, err := m.(*adaptive.Client).ReadScript(ctx, d.Id())
	if err != nil {
		if errors.Is(err, adaptive.ErrReadUnsupported) {
			// A newly added Optional+Computed map must have an empty baseline
			// on legacy state or the SDK plans a write solely to populate it.
			if _, exists := d.GetOk("parameter_descriptions"); !exists {
				if setErr := d.Set("parameter_descriptions", map[string]interface{}{}); setErr != nil {
					return diag.FromErr(setErr)
				}
			}
		}
		return readDiagnostics(d, err)
	}
	if _, full := data["commandOmitted"]; full {
		if _, exists := data["parameterDescriptions"]; !exists {
			data["parameterDescriptions"] = map[string]interface{}{}
		}
		if _, exists := data["description"]; !exists {
			data["description"] = ""
		}
	}
	if _, canonical := data["parameterDescriptions"]; !canonical {
		if _, legacy := data["ParameterDescriptions"]; !legacy {
			if _, prior := d.GetOk("parameter_descriptions"); !prior {
				data["parameterDescriptions"] = map[string]interface{}{}
			}
		}
	}
	fields := map[string][]string{
		"name": {"name", "Name"}, "endpoint": {"endpoint", "Endpoint"},
		"description": {"description", "Description"}, "help_text": {"helpText", "HelpText"},
		"parameter_descriptions": {"parameterDescriptions", "ParameterDescriptions"},
	}
	if omitted, _ := data["commandOmitted"].(bool); !omitted {
		fields["command"] = []string{"command", "Command"}
	}
	if diags := setResponseFields(d, data, fields); diags.HasError() {
		return diags
	}
	if digest, ok := data["commandDigest"]; ok {
		return refreshDigests(d, "command_digest", digest)
	}
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

	if _, err := client.UpdateScript(ctx, &scriptID, name, command, endpoint, scriptOptionsFromSchema(d)); err != nil {
		return diag.FromErr(err)
	}
	// Establish a new baseline on the next refresh after our own command write.
	return diag.FromErr(d.Set("command_digest", ""))
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

func scriptOptionsFromSchema(d *schema.ResourceData) adaptive.ScriptOptions {
	var options adaptive.ScriptOptions
	if v, ok := optionalValue(d, "description"); ok {
		value := v.(string)
		options.Description = &value
	}
	if v, ok := optionalValue(d, "help_text"); ok {
		value := v.(string)
		options.HelpText = &value
	}
	if v, ok := optionalValue(d, "parameter_descriptions"); ok {
		values := make(map[string]string)
		for k, v := range v.(map[string]interface{}) {
			values[k] = v.(string)
		}
		options.ParameterDescriptions = &values
	}
	return options
}
