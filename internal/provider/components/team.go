package components

import (
	"context"
	"fmt"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveTeam() *schema.Resource {
	return &schema.Resource{
		CreateContext: ResourceAdaptiveTeamCreate,
		ReadContext:   ResourceAdaptiveTeamRead,
		UpdateContext: ResourceAdaptiveTeamUpdate,
		DeleteContext: ResourceAdaptiveTeamDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the group. Must be unique.",
			},
			"members": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of emails to add to the team. If empty, the group will be created without members. ",
			},
			"endpoints": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of names of endpoints to add to this group. If empty, the group will be created without endpoints.",
			},
		},
	}
}

func ResourceAdaptiveTeamCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	mems := d.Get("members").([]interface{})
	members := make([]string, len(mems))
	for i, u := range mems {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			members[i] = val
		}
	}

	_endpoints := d.Get("endpoints").([]interface{})
	endpoints := make([]string, len(_endpoints))
	for i, u := range _endpoints {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("endpoint name must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("endpoint name cannot be empty"))
			}
			endpoints[i] = val
		}
	}

	resp, err := client.CreateTeam(ctx, name, &members, &endpoints)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	return nil
}

func ResourceAdaptiveTeamRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// client := m.(*adaptive.Client)

	// team, err := client.GetTeam(ctx, d.Id())
	// if err != nil {
	// 	return diag.FromErr(err)
	// }

	// if err := d.Set("name", team.Name); err != nil {
	// 	return diag.FromErr(err)
	// }
	// if err := d.Set("members", team.Members); err != nil {
	// 	return diag.FromErr(err)
	// }
	var diags diag.Diagnostics
	return diags
}

func ResourceAdaptiveTeamUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	teamID := d.Id()

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	mems := d.Get("members").([]interface{})
	members := make([]string, len(mems))
	for i, u := range mems {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			members[i] = val
		}
	}

	_endpoints := d.Get("endpoints").([]interface{})
	endpoints := make([]string, len(_endpoints))
	for i, u := range _endpoints {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("endpoint name must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("endpoint name cannot be empty"))
			}
			endpoints[i] = val
		}
	}

	if _, err := client.UpdateTeam(ctx, &teamID, name, &members, &endpoints); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func ResourceAdaptiveTeamDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	teamID := d.Id()

	if _, err := client.DeleteTeam(ctx, teamID, d.Get("name").(string)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
