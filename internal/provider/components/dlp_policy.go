package components

/*
resource "adaptive_dlp_policy" "example" {
	name                 = "pii-strict"
	block_screenshot     = true
	block_clipboard_copy = true
	block_bulk_export    = true
	max_export_rows      = 500
	applies_to_labels    = ["pii"]
}
*/

import (
	"context"
	"fmt"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveDLPPolicy() *schema.Resource {
	return &schema.Resource{
		Description: "Manages an Adaptive Data Loss Prevention (DLP) policy. The backend upserts " +
			"policies by name, so the policy name is used as the Terraform resource ID and renaming " +
			"a policy creates a new policy on the Adaptive server (the previously named policy is " +
			"deleted, which also clears any endpoint/workspace references to it).",
		CreateContext: ResourceAdaptiveDLPPolicyCreate,
		ReadContext:   ResourceAdaptiveDLPPolicyRead,
		UpdateContext: ResourceAdaptiveDLPPolicyUpdate,
		DeleteContext: ResourceAdaptiveDLPPolicyDelete,

		Importer: &schema.ResourceImporter{
			// The resource ID is the policy name, so `terraform import
			// adaptive_dlp_policy.example policy-name` works directly.
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				Description: "Name of the DLP policy. Must be unique within the workspace. Renaming " +
					"creates a new policy on the server (the old one is removed along with its " +
					"endpoint/workspace references).",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Human-readable description of the policy.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the policy is enforced. Defaults to true.",
			},
			"block_screenshot": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block screenshots of protected sessions.",
			},
			"block_clipboard_copy": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block copying data out of protected sessions via the clipboard.",
			},
			"block_clipboard_paste": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block pasting data into protected sessions via the clipboard.",
			},
			"block_file_transfer": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block file upload/download in protected sessions.",
			},
			"block_bulk_export": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block bulk data exports from protected sessions.",
			},
			"max_export_rows": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Maximum number of rows a single export may return when bulk export blocking is enabled. 0 means no row limit.",
				ValidateFunc: func(i interface{}, k string) ([]string, []error) {
					v, ok := i.(int)
					if !ok {
						return nil, []error{fmt.Errorf("%s must be an integer", k)}
					}
					if v < 0 {
						return nil, []error{fmt.Errorf("%s must be >= 0, got %d", k, v)}
					}
					return nil, nil
				},
			},
			"block_session_log_export": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Block exporting session logs/recordings.",
			},
			"blocked_surfaces": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Access surfaces the policy blocks entirely (e.g. webshell).",
			},
			"exceptions": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "expr-lang expressions describing sessions exempt from the policy " +
					"(e.g. `\"sre\" in teams && deviceTrusted`). Expressions are validated by the " +
					"Adaptive server; invalid expressions fail the apply with the server's message.",
			},
			"applies_to_labels": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Data labels the policy applies to (e.g. pii).",
			},
			"workspace_default": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether this policy is the workspace-wide default. Defaults to false.",
			},
			"policy_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Server-side identifier of the policy.",
			},
		},
	}
}

// dlpPolicyRequestFromSchema builds the flat API request from resource state.
func dlpPolicyRequestFromSchema(d *schema.ResourceData) (*adaptive.DLPPolicyRequest, error) {
	name := d.Get("name").(string)
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	return &adaptive.DLPPolicyRequest{
		Name:                  name,
		Description:           d.Get("description").(string),
		Enabled:               d.Get("enabled").(bool),
		BlockScreenshot:       d.Get("block_screenshot").(bool),
		BlockClipboardCopy:    d.Get("block_clipboard_copy").(bool),
		BlockClipboardPaste:   d.Get("block_clipboard_paste").(bool),
		BlockFileTransfer:     d.Get("block_file_transfer").(bool),
		BlockBulkExport:       d.Get("block_bulk_export").(bool),
		MaxExportRows:         d.Get("max_export_rows").(int),
		BlockSessionLogExport: d.Get("block_session_log_export").(bool),
		BlockedSurfaces:       stringList(d, "blocked_surfaces"),
		Exceptions:            stringList(d, "exceptions"),
		AppliesToLabels:       stringList(d, "applies_to_labels"),
		WorkspaceDefault:      d.Get("workspace_default").(bool),
	}, nil
}

func ResourceAdaptiveDLPPolicyCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	req, err := dlpPolicyRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.UpsertDLPPolicy(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}

	// The backend upserts by name, so the name is the stable handle Terraform
	// can read/delete with. Keep it as the resource ID (also enables
	// import-by-name).
	d.SetId(req.Name)
	if err := d.Set("policy_id", resp.ID); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveDLPPolicyRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	resp, err := client.GetDLPPolicy(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Policy was deleted out-of-band — drop it from state so Terraform recreates it.
	if resp == nil {
		d.SetId("")
		return nil
	}

	for key, value := range map[string]interface{}{
		"name":                     resp.Name,
		"description":              resp.Description,
		"enabled":                  resp.Enabled,
		"block_screenshot":         resp.BlockScreenshot,
		"block_clipboard_copy":     resp.BlockClipboardCopy,
		"block_clipboard_paste":    resp.BlockClipboardPaste,
		"block_file_transfer":      resp.BlockFileTransfer,
		"block_bulk_export":        resp.BlockBulkExport,
		"max_export_rows":          resp.MaxExportRows,
		"block_session_log_export": resp.BlockSessionLogExport,
		"blocked_surfaces":         resp.BlockedSurfaces,
		"exceptions":               resp.Exceptions,
		"applies_to_labels":        resp.AppliesToLabels,
		"workspace_default":        resp.WorkspaceDefault,
		"policy_id":                resp.ID,
	} {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func ResourceAdaptiveDLPPolicyUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	req, err := dlpPolicyRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.UpsertDLPPolicy(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}

	// The upsert is name-keyed, so a rename created a brand-new policy on the
	// server. Remove the previously named policy so it does not linger (the
	// backend also clears endpoint/workspace references to it).
	if d.HasChange("name") {
		oldName, _ := d.GetChange("name")
		if oldStr, ok := oldName.(string); ok && oldStr != "" && oldStr != req.Name {
			if _, err := client.DeleteDLPPolicy(ctx, oldStr); err != nil {
				return diag.FromErr(fmt.Errorf("renamed dlp policy %q created, but deleting the old policy %q failed: %w", req.Name, oldStr, err))
			}
		}
	}

	d.SetId(req.Name)
	if err := d.Set("policy_id", resp.ID); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveDLPPolicyDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	if _, err := client.DeleteDLPPolicy(ctx, d.Id()); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
