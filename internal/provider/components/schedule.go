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
		CreateContext: ResourceAdaptiveScheduleCreate,
		ReadContext:   ResourceAdaptiveScheduleRead,
		UpdateContext: ResourceAdaptiveScheduleUpdate,
		DeleteContext: ResourceAdaptiveScheduleDelete,

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
				Description: "Maximum access time in minutes for sessions approved under this schedule.",
			},
			"timezone": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "IANA timezone the window is evaluated in. Empty inherits the workspace default.",
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
	}

	if v := d.Get("expires_at").(string); v != "" {
		req.ExpiresAt = &v
	}
	if v := d.Get("max_access_time").(int); v > 0 {
		req.MaxAccessTime = &v
	}

	return req, nil
}

// stringList reads a TypeList of strings, rejecting empty entries.
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
	client := m.(*adaptive.Client)

	req, err := scheduleRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateSchedule(ctx, req)
	if err != nil {
		return diag.FromErr(err)
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
		d.SetId("")
		return nil
	}
	// The read endpoint returns a lossy view (no pattern fields), so we only
	// refresh attributes it authoritatively reports to avoid spurious diffs.
	if err := d.Set("name", resp.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("schedule_type", resp.ScheduleType); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("is_active", resp.IsActive); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("all_day", resp.AllDay); err != nil {
		return diag.FromErr(err)
	}
	if resp.Timezone != "" {
		if err := d.Set("timezone", resp.Timezone); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func ResourceAdaptiveScheduleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
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
