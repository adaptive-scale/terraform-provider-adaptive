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
		CustomizeDiff: familyInputPlan([]string{"name"}, []string{"members", "endpoints"}),
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
				DiffSuppressFunc: suppressTeamMembershipReorder("members"),
				Type:             schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of emails to add to the team. If empty, the group will be created without members. ",
			},
			"endpoints": {
				DiffSuppressFunc: suppressTeamMembershipReorder("endpoints"),
				Type:             schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of names of endpoints to add to this group. If empty, the group will be created without endpoints.",
			},
			"slack_channel_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Slack channel ID access requests for this group are routed to. A remote-only channel is not adopted on ordinary refresh; any group update sends the configured channel (empty when omitted), clearing an omitted remote channel. Import adopts the existing channel.",
			},
		},
	}
}

// suppressTeamMembershipReorder keeps the historical TypeList state encoding
// while comparing complete memberships, not individual positional elements.
func suppressTeamMembershipReorder(field string) schema.SchemaDiffSuppressFunc {
	return func(k, oldValue, newValue string, d *schema.ResourceData) bool {
		if k == field+".#" && oldValue != newValue {
			return false
		}
		old, next := d.GetChange(field)
		stringsFromList := func(value interface{}) []string {
			var out []string
			for _, item := range list(value) {
				out = append(out, str(item))
			}
			return out
		}
		return sameStringSet(stringsFromList(old), stringsFromList(next))
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
	if err := validateFamilyInputs(d.Get, nil, []string{"name"}, []string{"members", "endpoints"}); err != nil {
		return diag.FromErr(err)
	}
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
	if resp == nil || resp.ID == "" {
		return diag.Errorf("group create returned no trackable identity; inspect the remote object before retrying")
	}
	d.SetId(resp.ID)

	// The create route has no slack channel field, so a configured channel is
	// applied with a follow-up update.
	if slack := d.Get("slack_channel_id").(string); slack != "" {
		if err := client.UpdateTeam(ctx, resp.ID, *name, members, endpoints, slack); err != nil {
			// The group already exists. An error here taints a newly created
			// Terraform resource, causing a duplicate create on retry. Warn and
			// record the authoritative channel so a later plan retries Update.
			team, readErr := client.ReadTeam(ctx, resp.ID)
			channel := ""
			if readErr == nil && team != nil {
				channel = team.SlackChannelID
			}
			// Do not use the normal refresh callback: a transient post-create
			// 404 must not drop the ID acknowledged by the create response.
			_ = d.Set("slack_channel_id", channel)
			return diag.Diagnostics{{Severity: diag.Warning, Summary: "group created; Slack channel update needs retry", Detail: fmt.Sprintf("Group ID %s is retained. Run plan/apply again to retry the channel update without creating a duplicate group. Follow-up update: %v; read error: %v", resp.ID, err, readErr)}}
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
	if missingOnImport(d, isImport, team.Name) {
		return nil
	}

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
	if err := validateFamilyInputs(d.Get, nil, []string{"name"}, []string{"members", "endpoints"}); err != nil {
		return diag.FromErr(err)
	}
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
