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

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

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
			"slack_channel_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Slack channel ID access requests for this group are routed to.",
			},
		},
	}
}

// teamMembership reads the members and endpoints lists out of state, rejecting
// empty entries — the API silently ignores them, which would look like drift.
func teamMembership(d *schema.ResourceData) (members, endpoints []string, err error) {
	mems := d.Get("members").([]interface{})
	members = make([]string, len(mems))
	for i, u := range mems {
		val, ok := u.(string)
		if !ok {
			return nil, nil, fmt.Errorf("email must be a string")
		}
		if len(val) == 0 {
			return nil, nil, fmt.Errorf("email cannot be empty")
		}
		members[i] = val
	}

	_endpoints := d.Get("endpoints").([]interface{})
	endpoints = make([]string, len(_endpoints))
	for i, u := range _endpoints {
		val, ok := u.(string)
		if !ok {
			return nil, nil, fmt.Errorf("endpoint name must be a string")
		}
		if len(val) == 0 {
			return nil, nil, fmt.Errorf("endpoint name cannot be empty")
		}
		endpoints[i] = val
	}
	return members, endpoints, nil
}

func ResourceAdaptiveTeamCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	members, endpoints, err := teamMembership(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateTeam(ctx, *name, members, endpoints)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	// The create route has no slack channel field, so a configured channel is
	// applied with a follow-up update.
	if slack := d.Get("slack_channel_id").(string); slack != "" {
		if err := client.UpdateTeam(ctx, resp.ID, *name, members, endpoints, slack); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ResourceAdaptiveTeamRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	team, err := client.ReadTeam(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if team == nil {
		return notFoundGone(d)
	}
	isImport := isImportRead(d, "name")

	if err := d.Set("name", team.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("members", setPreservingOrder(stateStringSlice(d, "members"), team.Members)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("endpoints", setPreservingOrder(stateStringSlice(d, "endpoints"), team.Endpoints)); err != nil {
		return diag.FromErr(err)
	}
	// Don't adopt a server-side channel into a configuration that never set one,
	// but do report drift (including removal) for one that did.
	if prior := d.Get("slack_channel_id").(string); isImport || prior != "" {
		if err := d.Set("slack_channel_id", optString(prior, team.SlackChannelID, isImport)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ResourceAdaptiveTeamUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	teamID := d.Id()

	name, err := integrations.AttrFromSchema[string](d, "name", true)
	if err != nil {
		return diag.FromErr(err)
	}

	members, endpoints, err := teamMembership(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := client.UpdateTeam(ctx, teamID, *name, members, endpoints, d.Get("slack_channel_id").(string)); err != nil {
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
