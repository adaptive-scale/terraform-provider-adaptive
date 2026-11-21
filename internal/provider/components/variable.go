package components

import (
	"context"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveVariable() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveVariableCreate,
		ReadContext:   resourceAdaptiveVariableRead,
		UpdateContext: resourceAdaptiveVariableUpdate,
		DeleteContext: resourceAdaptiveVariableDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type: schema.TypeString, Required: true, ForceNew: true,
				Description: "Workspace-unique variable name. Use it in resource configuration as {{name}}. Changing it replaces the variable.",
			},
			"value": {
				Type: schema.TypeString, Required: true, Sensitive: true,
				Description: "Secret or configuration value substituted into resource configurations at runtime.",
			},
			"labels": {
				Type: schema.TypeSet, Optional: true, Elem: &schema.Schema{Type: schema.TypeString},
				Description: "Permission labels attached to the variable.",
			},
		},
	}
}

func variableRequest(d *schema.ResourceData) adaptive.VariableRequest {
	labels := []string{}
	if raw, ok := d.GetOk("labels"); ok {
		for _, item := range raw.(*schema.Set).List() {
			labels = append(labels, item.(string))
		}
	}
	return adaptive.VariableRequest{
		Name: d.Get("name").(string), Value: d.Get("value").(string), Labels: labels,
	}
}

func setVariableState(d *schema.ResourceData, variable *adaptive.VariableResponse) error {
	d.SetId(variable.ID)
	if err := d.Set("name", variable.Name); err != nil {
		return err
	}
	if err := d.Set("value", variable.Value); err != nil {
		return err
	}
	return d.Set("labels", variable.Labels)
}

func resourceAdaptiveVariableCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	variable, err := meta.(*adaptive.Client).CreateVariable(ctx, variableRequest(d))
	if err != nil {
		return diag.FromErr(err)
	}
	if err := setVariableState(d, variable); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAdaptiveVariableRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	variable, err := meta.(*adaptive.Client).GetVariable(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if variable == nil {
		d.SetId("")
		return nil
	}
	if err := setVariableState(d, variable); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAdaptiveVariableUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	variable, err := meta.(*adaptive.Client).UpdateVariable(ctx, d.Id(), variableRequest(d))
	if err != nil {
		return diag.FromErr(err)
	}
	if err := setVariableState(d, variable); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAdaptiveVariableDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := meta.(*adaptive.Client).DeleteVariable(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
