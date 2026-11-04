package components

import (
	"context"
	"fmt"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveSchedule() *schema.Resource {
	return &schema.Resource{
		CustomizeDiff: familyPlan(familyInputPlan([]string{"name", "schedule_type"}, []string{"users", "teams", "endpoints", "weekdays", "specific_dates"}), rejectExistingChange("timezone", scheduleTimezoneUpdateBlocked), rejectExistingClear("max_access_time", 0, scheduleDurationClearBlocked)),
		CreateContext: ResourceAdaptiveScheduleCreate,
		ReadContext:   ResourceAdaptiveScheduleRead,
		UpdateContext: ResourceAdaptiveScheduleUpdate,
		DeleteContext: ResourceAdaptiveScheduleDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the schedule. Must be unique within the workspace.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Human-readable description of the schedule.",
			},
			"schedule_type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "One of: weekdays, weekends, everyday, monthly, specific, custom.",
			},
			"is_active": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the schedule is active. Defaults to true.",
			},
			"all_day": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Run the full day, ignoring the start/end time-of-day fields.",
			},
			"start_hour": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Window start hour (0-23). Ignored when all_day is true.",
			},
			"start_minute": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Window start minute (0-59). Ignored when all_day is true.",
			},
			"end_hour": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Window end hour (0-23). Ignored when all_day is true.",
			},
			"end_minute": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Window end minute (0-59). Ignored when all_day is true.",
			},
			"weekdays": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "Weekday names (e.g. Monday). Used by schedule_type = custom.",
			},
			"start_day": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Start day of month (1-31). Used by schedule_type = monthly.",
			},
			"end_day": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "End day of month (1-31). Used by schedule_type = monthly.",
			},
			"specific_dates": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "RFC3339 timestamps. Used by schedule_type = specific.",
			},
			"users": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "User emails auto-approved by this schedule.",
			},
			"teams": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "Team names auto-approved by this schedule.",
			},
			"endpoints": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Description: "Endpoint (session) names this schedule applies to.",
			},
			"expires_at": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "RFC3339 instant after which the schedule stops applying.",
			},
			"max_access_time": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Maximum access time in minutes for sessions approved under this schedule. Removing an existing override is rejected: the backend preserves an omitted value, and a numeric zero is a real 0m TTL rather than an unset override.",
			},
			"timezone": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "IANA timezone the window is evaluated in. Empty inherits the workspace default on create. Changes and clears are rejected for existing schedules because the current backend update route does not persist timezone.",
			},
			"operation_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "autoapprove",
				Description: "Whether a request covered by this schedule is auto-approved or auto-rejected. One of: autoapprove (default), autoreject.",
				ValidateFunc: func(i interface{}, k string) ([]string, []error) {
					v, ok := i.(string)
					if !ok {
						return nil, []error{fmt.Errorf("%s must be a string", k)}
					}
					if v != "autoapprove" && v != "autoreject" {
						return nil, []error{fmt.Errorf("%s must be one of autoapprove, autoreject; got %q", k, v)}
					}
					return nil, nil
				},
			},
		},
	}
}

// scheduleRequestFromSchema builds the flat API request from resource state.
func scheduleRequestFromSchema(d *schema.ResourceData) (*adaptive.ScheduleRequest, error) {
	name := d.Get("name").(string)
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	scheduleType := d.Get("schedule_type").(string)
	if scheduleType == "" {
		return nil, fmt.Errorf("schedule_type cannot be empty")
	}

	isActive := d.Get("is_active").(bool)
	req := &adaptive.ScheduleRequest{
		Name:          name,
		Description:   d.Get("description").(string),
		ScheduleType:  scheduleType,
		IsActive:      &isActive,
		AllDay:        d.Get("all_day").(bool),
		StartHour:     d.Get("start_hour").(int),
		StartMinute:   d.Get("start_minute").(int),
		EndHour:       d.Get("end_hour").(int),
		EndMinute:     d.Get("end_minute").(int),
		StartDay:      d.Get("start_day").(int),
		EndDay:        d.Get("end_day").(int),
		Weekdays:      stringList(d, "weekdays"),
		SpecificDates: stringList(d, "specific_dates"),
		Users:         stringList(d, "users"),
		Teams:         stringList(d, "teams"),
		Endpoints:     stringList(d, "endpoints"),
		Timezone:      d.Get("timezone").(string),
		OperationType: d.Get("operation_type").(string),
	}

	if v := d.Get("expires_at").(string); v != "" {
		req.ExpiresAt = &v
	}
	if v := d.Get("max_access_time").(int); v > 0 {
		req.MaxAccessTime = &v
	}

	return req, nil
}

// stringList reads a TypeList of strings. Plan and apply input guards reject
// blank entries before this builder is called.
func stringList(d *schema.ResourceData, key string) []string {
	raw := d.Get(key).([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func ResourceAdaptiveScheduleCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "schedule_type"}, []string{"users", "teams", "endpoints", "weekdays", "specific_dates"}); err != nil {
		return diag.FromErr(err)
	}
	client := m.(*adaptive.Client)

	req, err := scheduleRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateSchedule(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}
	if resp == nil || resp.ID == "" {
		return diag.Errorf("schedule create returned no trackable identity; inspect the remote object before retrying")
	}
	d.SetId(resp.ID)
	return nil
}

func ResourceAdaptiveScheduleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	resp, err := client.GetSchedule(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Schedule was deleted out-of-band — drop it from state so Terraform recreates it.
	if resp == nil {
		return notFoundGone(d)
	}
	if missingOnImport(d, isImportRead(d, "name"), resp.Name) {
		return nil
	}
	// The current DTO always emits its window and canonical membership fields.
	// This presence signature makes omitted optional fields authoritative clears.
	// Legacy/partial DTOs instead refresh only fields actually present and warn.
	if err := d.Set("name", resp.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("schedule_type", resp.ScheduleType); err != nil {
		return diag.FromErr(err)
	}
	if resp.HasField("isActive") {
		if err := d.Set("is_active", resp.IsActive); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.HasField("allDay") {
		if err := d.Set("all_day", resp.AllDay); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.FullRead() || resp.HasField("timezone") {
		if err := d.Set("timezone", resp.Timezone); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.OperationType != "" {
		if err := d.Set("operation_type", resp.OperationType); err != nil {
			return diag.FromErr(err)
		}
	}
	// Preserve equivalent RFC3339 spellings, but honor a modern empty expiry.
	// Missing legacy expiry remains unknown rather than evidence of removal.
	if (resp.FullRead() || resp.HasField("expiresAt")) && !sameInstantSet([]string{d.Get("expires_at").(string)}, []string{resp.ExpiresAt}) {
		if err := d.Set("expires_at", resp.ExpiresAt); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.FullRead() || resp.HasField("description") {
		if err := d.Set("description", resp.Description); err != nil {
			return diag.FromErr(err)
		}
	}

	// The time-of-day window travels as four independent ints, so it is only
	// refreshed only when all four values are present, including known zeroes.
	if resp.HasField("startHour") && resp.HasField("startMinute") && resp.HasField("endHour") && resp.HasField("endMinute") {
		for key, v := range map[string]int{
			"start_hour":   resp.StartHour,
			"start_minute": resp.StartMinute,
			"end_hour":     resp.EndHour,
			"end_minute":   resp.EndMinute,
		} {
			if err := d.Set(key, v); err != nil {
				return diag.FromErr(err)
			}
		}
	}
	if resp.FullRead() || resp.HasField("startDay") {
		if err := d.Set("start_day", resp.StartDay); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.FullRead() || resp.HasField("endDay") {
		if err := d.Set("end_day", resp.EndDay); err != nil {
			return diag.FromErr(err)
		}
	}
	if resp.FullRead() || resp.HasField("maxAccessTime") {
		value := 0
		if resp.MaxAccessTime != nil {
			value = *resp.MaxAccessTime
		}
		if err := d.Set("max_access_time", value); err != nil {
			return diag.FromErr(err)
		}
	}

	// The backend lowercases weekday names, so only a genuine membership
	// change — not a spelling difference — is written back.
	if (resp.FullRead() || resp.HasField("weekdays")) && !sameStringSetFold(stringList(d, "weekdays"), resp.Weekdays) {
		if err := d.Set("weekdays", resp.Weekdays); err != nil {
			return diag.FromErr(err)
		}
	}
	// specific_dates come back normalized to UTC; the configuration's spelling
	// of the same instants is kept.
	if (resp.FullRead() || resp.HasField("specificDates")) && !sameInstantSet(stringList(d, "specific_dates"), resp.SpecificDates) {
		if err := d.Set("specific_dates", resp.SpecificDates); err != nil {
			return diag.FromErr(err)
		}
	}
	for key, server := range map[string][]string{
		"users":     resp.Users,
		"teams":     resp.Teams,
		"endpoints": resp.EndpointNames(),
	} {
		if (resp.FullRead() || resp.HasField(key)) && !sameStringSet(stringList(d, key), server) {
			if err := d.Set(key, server); err != nil {
				return diag.FromErr(err)
			}
		}
	}
	if !resp.FullRead() {
		return diag.Diagnostics{{Severity: diag.Warning, Summary: "schedule read capabilities are incomplete", Detail: "This backend returned a legacy or partial schedule view. Absent fields retain prior state; removals and drift on those fields cannot be verified. mappedEndpoints IDs are not adopted as endpoint names. Upgrade to the full schedule read DTO before relying on complete refresh/import coverage."}}
	}
	return nil
}

func ResourceAdaptiveScheduleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := validateFamilyInputs(d.Get, nil, []string{"name", "schedule_type"}, []string{"users", "teams", "endpoints", "weekdays", "specific_dates"}); err != nil {
		return diag.FromErr(err)
	}
	if d.HasChange("max_access_time") && d.Get("max_access_time") == 0 {
		return diag.Errorf("max_access_time cannot be cleared: %s", scheduleDurationClearBlocked)
	}
	if d.HasChange("timezone") {
		return diag.Errorf("timezone cannot be updated: %s", scheduleTimezoneUpdateBlocked)
	}
	client := m.(*adaptive.Client)

	req, err := scheduleRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if _, err := client.UpdateSchedule(ctx, d.Id(), req); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveScheduleDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	if _, err := client.DeleteSchedule(ctx, d.Id(), d.Get("name").(string)); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
