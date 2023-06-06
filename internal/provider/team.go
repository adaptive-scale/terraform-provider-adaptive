package provider

import (
	"context"
	"fmt"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAdaptiveTeam() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveTeamCreate,
		ReadContext:   resourceAdaptiveTeamRead,
		UpdateContext: resourceAdaptiveTeamUpdate,
		DeleteContext: resourceAdaptiveTeamDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			},
			"members": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of emails to add to the team. They should be  . If empty, the team will be created without members. ",
			},
		},
	}
}

func resourceAdaptiveTeamCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	name, err := attrFromSchema[string](d, "name", true)
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

	resp, err := client.CreateTeam(ctx, name, &members)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	return nil
}

func resourceAdaptiveTeamRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
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

func resourceAdaptiveTeamUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	teamID := d.Id()

	name, err := attrFromSchema[string](d, "name", true)
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

	if _, err := client.UpdateTeam(ctx, &teamID, name, &members); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceAdaptiveTeamDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	teamID := d.Id()

	if _, err := client.DeleteTeam(ctx, teamID); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
