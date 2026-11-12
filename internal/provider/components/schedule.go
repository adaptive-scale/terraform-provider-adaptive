package components

import (
	"context"
	"fmt"
	"strings"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceAdaptiveSchedule() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages scheduled access approval or rejection, including time windows, users, groups and endpoints.",
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
	data, err := m.(*adaptive.Client).ReadSchedule(ctx, d.Id())
	if err != nil {
		return readDiagnostics(d, err)
	}
	fields := map[string][]string{
		"name": {"name"}, "description": {"description"}, "schedule_type": {"scheduleType"},
		"is_active": {"isActive"}, "all_day": {"allDay"}, "start_hour": {"startHour"},
		"start_minute": {"startMinute"}, "end_hour": {"endHour"}, "end_minute": {"endMinute"},
		"weekdays": {"weekdays"}, "start_day": {"startDay"}, "end_day": {"endDay"},
		"specific_dates": {"specificDates"}, "users": {"users"}, "teams": {"teams"},
		"endpoints": {"endpoints"}, "expires_at": {"expiresAt"}, "max_access_time": {"maxAccessTime"},
		"timezone": {"timezone"}, "operation_type": {"operationType"},
	}
	// Pattern fields distinguish the full v0.9 view from the old envelope-only
	// view. Only the full view authoritatively omits cleared optional values.
	if _, full := data["startHour"]; full {
		for _, key := range []string{"description", "expiresAt", "timezone"} {
			if _, ok := data[key]; !ok {
				data[key] = ""
			}
		}
		for _, key := range []string{"startDay", "endDay", "maxAccessTime"} {
			if _, ok := data[key]; !ok {
				data[key] = 0
			}
		}
		for _, key := range []string{"weekdays", "specificDates"} {
			if _, ok := data[key]; !ok {
				data[key] = []interface{}{}
			}
		}
	}
	// All-day patterns are normalized to 00:00-23:59 by the server; the
	// ignored time fields must retain the user's previous representation.
	if allDay, _ := data["allDay"].(bool); allDay && d.Get("all_day").(bool) {
		for _, key := range []string{"start_hour", "start_minute", "end_hour", "end_minute"} {
			delete(fields, key)
		}
	}
	if incoming, ok := data["weekdays"].([]interface{}); ok {
		for i, day := range incoming {
			for _, old := range d.Get("weekdays").([]interface{}) {
				if strings.EqualFold(fmt.Sprint(day), fmt.Sprint(old)) {
					incoming[i] = old
					break
				}
			}
		}
	}
	if incoming, ok := data["specificDates"].([]interface{}); ok {
		for i, date := range incoming {
			for _, old := range d.Get("specific_dates").([]interface{}) {
				if equivalentString("expires_at", fmt.Sprint(old), fmt.Sprint(date)) {
					incoming[i] = old
					break
				}
			}
		}
	}
	return setResponseFields(d, data, fields)
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
