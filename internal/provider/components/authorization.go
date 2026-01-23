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
	"kubernetes":                  true,
	"mongo":                       true,
	"mongodb_atlas":               true,
	"mongodb_aws_secrets_manager": true,
	"mongo36":                     true,
	"elasticsearch":               true,
	"ssh":                         true,
	"postgres":                    true,
	"mysql":                       true,
	"sql_server":                  true,
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
		CreateContext: ResourceAdaptiveAuthorizationCreate,
		ReadContext:   ResourceAdaptiveAuthorizationRead,
		UpdateContext: ResourceAdaptiveAuthorizationUpdate,
		DeleteContext: ResourceAdaptiveAuthorizationDelete,

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
				Description: "An optional description of the authorization object.",
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
	client := m.(*adaptive.Client)

	obj := SchemaToAuthorizationConfiguration(d)
	// config, err := json.Marshal(obj)
	// if err != nil {
	// 	return diag.FromErr(fmt.Errorf("could not marshal resource configuration %w", err))
	// }

	resp, err := client.CreateAuthorization(ctx, obj.Name, obj.Description, obj.Permission, obj.ResourceType)
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

	resp, err := client.ReadAuthorization(ctx, authID, false)
	if err != nil {
		return diag.FromErr(err)
	}

	data, ok := resp.(map[string]interface{})
	if !ok {
		return diag.Errorf("invalid response format")
	}

	if name, ok := data["name"].(string); ok {
		d.Set("name", name)
	}
	if description, ok := data["description"].(string); ok {
		d.Set("description", description)
	}
	if resourceType, ok := data["resource_type"].(string); ok {
		d.Set("resource_type", resourceType)
	}
	if permissions, ok := data["permissions"].(string); ok {
		d.Set("permissions", permissions)
	}

	return nil
}

func ResourceAdaptiveAuthorizationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
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
