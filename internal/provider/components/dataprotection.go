package components

import (
	"context"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func ResourceAdaptiveDataProtection() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages masking on an Adaptive resource. Destroy disables masking; the server retains the policy, generated authorization and masked views.",
		CreateContext: resourceAdaptiveDataProtectionWrite,
		ReadContext:   resourceAdaptiveDataProtectionRead,
		UpdateContext: resourceAdaptiveDataProtectionWrite,
		DeleteContext: resourceAdaptiveDataProtectionDelete,
		Importer:      &schema.ResourceImporter{StateContext: schema.ImportStatePassthroughContext},
		Schema: map[string]*schema.Schema{
			"resource": {Type: schema.TypeString, Required: true, ForceNew: true, ValidateFunc: validation.StringIsNotWhiteSpace, Description: "Name of the resource whose masking policy is managed. Only one policy can be managed per resource."},
			"scoped":   {Type: schema.TypeBool, Optional: true, Default: true, Description: "Restrict sessions to masked views."},
			"masks": {Type: schema.TypeList, Optional: true, Description: "Database masking rules. An empty list removes the rules.", Elem: &schema.Resource{Schema: map[string]*schema.Schema{
				"database_name": {Type: schema.TypeString, Required: true, ValidateFunc: validation.StringIsNotWhiteSpace},
				"tables": {Type: schema.TypeList, Optional: true, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"table_name": {Type: schema.TypeString, Required: true, ValidateFunc: validation.StringIsNotWhiteSpace},
					"schema":     {Type: schema.TypeString, Optional: true},
					"masked":     {Type: schema.TypeBool, Optional: true, Default: false},
					"masked_columns": {Type: schema.TypeList, Optional: true, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"column_name":  {Type: schema.TypeString, Required: true, ValidateFunc: validation.StringIsNotWhiteSpace},
						"masking_type": {Type: schema.TypeString, Required: true, ValidateFunc: validation.StringIsNotWhiteSpace, Description: "Masking type supported by the Adaptive server. The server validates available types."},
					}}},
				}}},
			}}},
			"authorization_name": {Type: schema.TypeString, Computed: true, Description: "Generated authorization name to use on masked endpoints."},
			"status":             {Type: schema.TypeString, Computed: true, Description: "Server-reported policy status."},
			"last_applied_at":    {Type: schema.TypeString, Computed: true, Description: "Last application timestamp."},
		},
	}
}

func dataProtectionFromSchema(d *schema.ResourceData) *adaptive.DataProtectionRequest {
	result := &adaptive.DataProtectionRequest{ResourceName: d.Get("resource").(string), Scoped: d.Get("scoped").(bool), Masks: []adaptive.DataProtectionMask{}}
	for _, raw := range d.Get("masks").([]interface{}) {
		mask := raw.(map[string]interface{})
		m := adaptive.DataProtectionMask{DatabaseName: mask["database_name"].(string), Tables: []adaptive.DataProtectionTable{}}
		for _, raw := range mask["tables"].([]interface{}) {
			table := raw.(map[string]interface{})
			t := adaptive.DataProtectionTable{TableName: table["table_name"].(string), Schema: table["schema"].(string), Masked: table["masked"].(bool), MaskedColumns: []adaptive.DataProtectionColumn{}}
			for _, raw := range table["masked_columns"].([]interface{}) {
				column := raw.(map[string]interface{})
				t.MaskedColumns = append(t.MaskedColumns, adaptive.DataProtectionColumn{ColumnName: column["column_name"].(string), MaskingType: column["masking_type"].(string)})
			}
			m.Tables = append(m.Tables, t)
		}
		result.Masks = append(result.Masks, m)
	}
	return result
}

func setDataProtection(d *schema.ResourceData, response *adaptive.DataProtectionResponse) diag.Diagnostics {
	masks := make([]interface{}, 0, len(response.Masks))
	for _, mask := range response.Masks {
		tables := make([]interface{}, 0, len(mask.Tables))
		for _, table := range mask.Tables {
			columns := make([]interface{}, 0, len(table.MaskedColumns))
			for _, column := range table.MaskedColumns {
				columns = append(columns, map[string]interface{}{"column_name": column.ColumnName, "masking_type": column.MaskingType})
			}
			tables = append(tables, map[string]interface{}{"table_name": table.TableName, "schema": table.Schema, "masked": table.Masked, "masked_columns": columns})
		}
		masks = append(masks, map[string]interface{}{"database_name": mask.DatabaseName, "tables": tables})
	}
	for key, value := range map[string]interface{}{
		"resource": response.ResourceName, "scoped": response.Scoped, "masks": masks,
		"authorization_name": response.AuthorizationName, "status": response.Status, "last_applied_at": response.LastAppliedAt,
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourceAdaptiveDataProtectionWrite(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resp, err := m.(*adaptive.Client).WriteDataProtection(ctx, d.Id(), dataProtectionFromSchema(d))
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)
	return setDataProtection(d, resp)
}

func resourceAdaptiveDataProtectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resp, err := m.(*adaptive.Client).ReadDataProtection(ctx, d.Id())
	if err != nil {
		return readDiagnostics(d, err)
	}
	return setDataProtection(d, resp)
}

func resourceAdaptiveDataProtectionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := m.(*adaptive.Client).DeleteDataProtection(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
