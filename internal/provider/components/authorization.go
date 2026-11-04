package components

/*
resource "adaptive_authorization" "example" {
	name = "instance-name"
	description = ""
	resource = ""
	permission - ""
*/

import (
	"context"
	"strings"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var validAuthorizedResourceTypes = map[string]bool{
	"kubernetes":                    true,
	"mongodb":                       true,
	"mongodb_atlas":                 true,
	"mongodb_aws_secrets_manager":   true,
	"mongodb36":                     true, // backend model.Mongo36; "mongo36" was never canonical
	"elasticsearch":                 true,
	"ssh":                           true,
	"postgres":                      true,
	"mysql":                         true,
	"sql_server":                    true,
	"sqlserver_aws_secrets_manager": true,
	"postgres_aws_secrets_manager":  true,
	"mysql_aws_secrets_manager":     true,
	"yugabytedb":                    true,
	"cockroachdb":                   true,
	"proxysql":                      true,
}

func validateAuthorizedResourceType(i any, p cty.Path) diag.Diagnostics {
	v, ok := i.(string)
	if !ok {
		return diag.Errorf("expected resource_type to be string")
	}

	if !validAuthorizedResourceTypes[v] {
		validTypes := make([]string, 0, len(validAuthorizedResourceTypes))
		for k := range validAuthorizedResourceTypes {
			validTypes = append(validTypes, k)
		}
		return diag.Errorf("invalid resource_type %q; valid types are: %s", v, strings.Join(validTypes, ", "))
	}

	return nil
}

type AuthorizationConfiguration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Authorization model has changed since v0.4.0
	//Resource    string `json:"resource"`
	ResourceType string `json:"resource_type"`
	Permission   string `json:"permission"`
}

func ResourceAdaptiveAuthorization() *schema.Resource {
	return &schema.Resource{
		CustomizeDiff: familyPlan(familyInputPlan([]string{"name", "resource_type", "permissions"}, nil), rejectAuthorizationResourceTypeChange, rejectExistingClear("description", "", descriptionClearBlocked)),
		CreateContext: ResourceAdaptiveAuthorizationCreate,
		ReadContext:   ResourceAdaptiveAuthorizationRead,
		UpdateContext: ResourceAdaptiveAuthorizationUpdate,
		DeleteContext: ResourceAdaptiveAuthorizationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the authorization object.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "An optional description of the authorization object. The current backend preserves empty descriptions on update, so clearing an existing description is rejected at plan until backend support is available.",
			},
			"resource_type": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "Resource type to grant permission on. Eg. kubernetes, postgres, mysql, mongodb",
				ValidateDiagFunc: validateAuthorizedResourceType,
			},
			"permissions": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The permission to grant or revoke on the specified resource.",
			},
		},
	}
}

func SchemaToAuthorizationConfiguration(d *schema.ResourceData) AuthorizationConfiguration {
	return AuthorizationConfiguration{
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		ResourceType: d.Get("resource_type").(string),
		Permission:   d.Get("permissions").(string),
	}
}

func ResourceAdaptiveAuthorizationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "resource_type", "permissions"}, nil); err != nil {
		return diag.FromErr(err)
	}
	client := m.(*adaptive.Client)

	obj := SchemaToAuthorizationConfiguration(d)
	// config, err := json.Marshal(obj)
	// if err != nil {
	// 	return diag.FromErr(fmt.Errorf("could not marshal resource configuration %w", err))
	// }

	resp, err := client.CreateAuthorization(ctx, obj.Name, obj.Description, obj.Permission, obj.ResourceType)
	if resp != nil && resp.ID != "" {
		d.SetId(resp.ID)
	}
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	// ResourceAdaptiveAuthorizationRead(ctx, d, m)
	return nil
}

func ResourceAdaptiveAuthorizationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	authID := d.Id()

	resp, err := client.ReadAuthorization(ctx, authID)
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if resp == nil {
		return notFoundGone(d)
	}
	if missingOnImport(d, isImportRead(d, "name"), resp.Name) {
		return nil
	}

	if isImportRead(d, "name") && (!resp.ResourceTypePresent || resp.ResourceType == "") {
		return diag.Errorf("authorization import cannot determine resource_type: the backend omitted or returned an empty type; upgrade the read DTO or supply the known resource_type in configuration before refreshing")
	}

	if err := d.Set("name", resp.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", resp.Description); err != nil {
		return diag.FromErr(err)
	}
	if resp.ResourceTypePresent {
		if err := d.Set("resource_type", resp.ResourceType); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := d.Set("permissions", resp.Permissions); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func ResourceAdaptiveAuthorizationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if d.Id() != "" && d.HasChange("resource_type") {
		return diag.Errorf("resource_type cannot be updated: %s", authorizationResourceTypeUpdateBlocked)
	}
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "resource_type", "permissions"}, nil); err != nil {
		return diag.FromErr(err)
	}
	if d.HasChange("description") && d.Get("description") == "" {
		return diag.Errorf("description cannot be cleared: %s", descriptionClearBlocked)
	}
	client := m.(*adaptive.Client)
	authID := d.Id()

	//if d.HasChange("permissions") {
	//	return diag.Errorf("Cannot change permission of authorization after published")
	//}
	//if d.HasChange("resource_type") {
	//	return diag.Errorf("Cannot change resource of authorization after published")
	//}

	obj := SchemaToAuthorizationConfiguration(d)

	_, err := client.UpdateAuthorization(ctx, authID, obj.Name, obj.Description, obj.Permission, obj.ResourceType)
	if err != nil {
		return diag.FromErr(err)
	}

	// ResourceAdaptiveAuthorizationRead(ctx, d, m)
	return nil
}

func ResourceAdaptiveAuthorizationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteAuthorization(ctx, resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
