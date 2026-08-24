package components

import (
	"context"
	"fmt"
	"sort"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
)

var validMaskingTypes = []string{
	"name", "email", "ssn", "phone", "cc", "aadhar",
	"first4", "last4", "redact", "hash", "disable",
}

func ResourceAdaptiveDataProtection() *schema.Resource {
	return &schema.Resource{
		Description: "Column-level masking policy for a database resource. Creating a policy for a resource " +
			"that already has one (for example one configured in the UI) adopts and overwrites it rather than failing. " +
			"Destroying it turns masking off; the generated `masked_<resource>` authorization and any masked views " +
			"that were already provisioned are left in place.",

		CreateContext: ResourceAdaptiveDataProtectionCreate,
		ReadContext:   ResourceAdaptiveDataProtectionRead,
		UpdateContext: ResourceAdaptiveDataProtectionUpdate,
		DeleteContext: ResourceAdaptiveDataProtectionDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"resource": {
				Type:     schema.TypeString,
				Required: true,
				Description: "Name of the resource (integration) this masking policy protects. Cannot be changed " +
					"after creation.",
			},
			"scoped": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				Description: "When true (the default), session users only see the generated masked views. When " +
					"false, users see everything except the raw masked tables, which requires synced metadata.",
			},
			"masks": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Masking rules, grouped per database and table.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"database_name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Database the rules below apply to.",
						},
						"tables": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Per-table masking rules.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"table_name": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Name of the table.",
									},
									"schema": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Schema the table lives in. Defaults to the database's default schema.",
									},
									"masked": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether masking is applied to this table.",
									},
									"masked_columns": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Columns to mask, and how.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"column_name": {
													Type:        schema.TypeString,
													Required:    true,
													Description: "Name of the column to mask.",
												},
												"masking_type": {
													Type:     schema.TypeString,
													Required: true,
													Description: "How the column is masked. One of: " +
														"name, email, ssn, phone, cc, aadhar, first4, last4, redact, hash, disable.",
													ValidateFunc: func(i interface{}, k string) ([]string, []error) {
														v, ok := i.(string)
														if !ok {
															return nil, []error{fmt.Errorf("%s must be a string", k)}
														}
														if !slices.Contains(validMaskingTypes, v) {
															return nil, []error{fmt.Errorf("%s must be one of %v; got %q", k, validMaskingTypes, v)}
														}
														return nil, nil
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"authorization_name": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Name of the generated masking authorization (`masked_<resource>`). Attach it to an " +
					"endpoint's `authorization` to serve masked sessions.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Provisioning status of the policy. Masked views are created lazily at session start.",
			},
			"last_applied_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Last time the policy was applied to the target database.",
			},
		},
	}
}

// dataProtectionRequestFromSchema builds the API request from resource state.
func dataProtectionRequestFromSchema(d *schema.ResourceData) adaptive.DataProtectionRequest {
	scoped := d.Get("scoped").(bool)
	return adaptive.DataProtectionRequest{
		ResourceName: d.Get("resource").(string),
		Scoped:       &scoped,
		Masks:        masksFromSchema(d.Get("masks").([]interface{})),
	}
}

func masksFromSchema(raw []interface{}) []adaptive.DataProtectionMask {
	masks := make([]adaptive.DataProtectionMask, 0, len(raw))
	for _, m := range raw {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		mask := adaptive.DataProtectionMask{DatabaseName: str(mm["database_name"])}
		for _, t := range list(mm["tables"]) {
			tm, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			masked := boolean(tm["masked"])
			table := adaptive.DataProtectionTable{
				TableName: str(tm["table_name"]),
				Schema:    str(tm["schema"]),
				Masked:    &masked,
			}
			for _, c := range list(tm["masked_columns"]) {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				table.MaskedColumns = append(table.MaskedColumns, adaptive.DataProtectionColumnMask{
					ColumnName:  str(cm["column_name"]),
					MaskingType: str(cm["masking_type"]),
				})
			}
			mask.Tables = append(mask.Tables, table)
		}
		masks = append(masks, mask)
	}
	return masks
}

// masksToSchema is the inverse of masksFromSchema.
func masksToSchema(masks []adaptive.DataProtectionMask) []interface{} {
	out := make([]interface{}, 0, len(masks))
	for _, m := range masks {
		tables := make([]interface{}, 0, len(m.Tables))
		for _, t := range m.Tables {
			cols := make([]interface{}, 0, len(t.MaskedColumns))
			for _, c := range t.MaskedColumns {
				cols = append(cols, map[string]interface{}{
					"column_name":  c.ColumnName,
					"masking_type": c.MaskingType,
				})
			}
			tables = append(tables, map[string]interface{}{
				"table_name":     t.TableName,
				"schema":         t.Schema,
				"masked":         t.Masked != nil && *t.Masked,
				"masked_columns": cols,
			})
		}
		out = append(out, map[string]interface{}{
			"database_name": m.DatabaseName,
			"tables":        tables,
		})
	}
	return out
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func boolean(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func list(v interface{}) []interface{} {
	l, _ := v.([]interface{})
	return l
}

// setDataProtectionOutputs copies the server-computed fields onto the state.
func setDataProtectionOutputs(d *schema.ResourceData, r *adaptive.DataProtectionReadResponse) error {
	if err := d.Set("authorization_name", r.AuthorizationName); err != nil {
		return err
	}
	if err := d.Set("status", r.Status); err != nil {
		return err
	}
	return d.Set("last_applied_at", r.LastAppliedAt)
}

func ResourceAdaptiveDataProtectionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	resp, err := client.CreateDataProtection(ctx, dataProtectionRequestFromSchema(d))
	if err != nil {
		return diag.FromErr(err)
	}
	// The ID is the protected resource's integration id; without it there is
	// nothing to track, and an empty ID would read as "never created".
	if resp.ID == "" {
		return diag.Errorf("data protection policy for %q was written but the server returned no resource id",
			d.Get("resource").(string))
	}
	d.SetId(resp.ID)
	if err := setDataProtectionOutputs(d, resp); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveDataProtectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	r, err := client.ReadDataProtection(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if r == nil {
		return notFoundGone(d)
	}
	if missingOnImport(d, isImportRead(d, "resource"), r.ResourceName) {
		return nil
	}

	if err := d.Set("resource", r.ResourceName); err != nil {
		return diag.FromErr(err)
	}

	// scoped defaults to true server-side, so an unset attribute reading back
	// true is not drift; false must always surface.
	serverScoped := r.Scoped == nil || *r.Scoped
	if d.Get("scoped").(bool) != serverScoped {
		if err := d.Set("scoped", serverScoped); err != nil {
			return diag.FromErr(err)
		}
	}

	// Keep the configuration's ordering and spelling while the two describe the
	// same policy; otherwise take the server's view.
	if !sameMasks(masksFromSchema(d.Get("masks").([]interface{})), r.Masks) {
		if err := d.Set("masks", masksToSchema(r.Masks)); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := setDataProtectionOutputs(d, r); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveDataProtectionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	// The policy is bound to its resource; moving it means a new policy.
	if d.HasChange("resource") {
		return diag.Errorf("resource cannot be updated for an existing data protection policy")
	}

	resp, err := client.UpdateDataProtection(ctx, d.Id(), dataProtectionRequestFromSchema(d))
	if err != nil {
		return diag.FromErr(err)
	}
	if resp != nil {
		if err := setDataProtectionOutputs(d, resp); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

// ResourceAdaptiveDataProtectionDelete turns masking off (the server applies the
// documented "off" shape: scoped false, no masks). The masked_<resource>
// authorization and any already-provisioned masked views are left in place.
func ResourceAdaptiveDataProtectionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	if err := client.DeleteDataProtection(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

// sameMasks compares two mask sets structurally, ignoring the ordering of
// databases, tables and columns.
func sameMasks(a, b []adaptive.DataProtectionMask) bool {
	if len(a) != len(b) {
		return false
	}
	return maskKey(a) == maskKey(b)
}

func maskKey(masks []adaptive.DataProtectionMask) string {
	dbs := make([]string, 0, len(masks))
	for _, m := range masks {
		tables := make([]string, 0, len(m.Tables))
		for _, t := range m.Tables {
			cols := make([]string, 0, len(t.MaskedColumns))
			for _, c := range t.MaskedColumns {
				cols = append(cols, c.ColumnName+"="+c.MaskingType)
			}
			sort.Strings(cols)
			masked := t.Masked != nil && *t.Masked
			tables = append(tables, fmt.Sprintf("%s|%s|%t|%v", t.TableName, t.Schema, masked, cols))
		}
		sort.Strings(tables)
		dbs = append(dbs, fmt.Sprintf("%s>>%v", m.DatabaseName, tables))
	}
	sort.Strings(dbs)
	return fmt.Sprintf("%v", dbs)
}
