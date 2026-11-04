package components

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestSchedulePartialDTOCannotClaimFullRead(t *testing.T) {
	client := scheduleReadServer(t, `{"id":"s-1","name":"window","scheduleType":"weekdays","users":[],"teams":[],"endpoints":[],"startHour":0,"startMinute":0,"endHour":0,"endMinute":0}`)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "description": "preserve-unknown"})
	d.SetId("s-1")
	diags := ResourceAdaptiveScheduleRead(context.Background(), d, client)
	if diags.HasError() || len(diags) == 0 || d.Get("description").(string) != "preserve-unknown" {
		t.Fatal("missing required DTO flags falsely established full-read capability", diags)
	}
}

func TestFamiliesCreateRejectMissingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		r      *schema.Resource
		values map[string]interface{}
	}{
		{"group", ResourceAdaptiveTeam(), map[string]interface{}{"name": "ops"}},
		{"script", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "endpoint": "db", "command": "echo test"}},
		{"schedule", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
			d := schema.TestResourceDataRaw(t, tc.r.Schema, tc.values)
			if diags := tc.r.CreateContext(context.Background(), d, client); !diags.HasError() {
				t.Fatal("missing create identity reported as success")
			}
			if len(api.paths) != 1 {
				t.Fatal("untrackable create identity reached a follow-up request", api.paths)
			}
		})
	}
}

func TestScheduleExpiryKeepsEquivalentInstantSpelling(t *testing.T) {
	client := scheduleReadServer(t, `{"id":"s-1","name":"window","scheduleType":"weekdays","expiresAt":"2030-01-01T00:00:00Z"}`)
	const configured = "2030-01-01T05:30:00+05:30"
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "expires_at": configured})
	d.SetId("s-1")
	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Get("expires_at").(string) != configured {
		t.Fatal("equivalent expiry normalization would cause perpetual plan drift")
	}
}

func TestTeamSlackFollowupReadNotFoundRetainsCreatedID(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/terraform/team/create":
			w.Write([]byte(`{"id":"t-1"}`))
		case "/api/v1/terraform/team/update/t-1":
			http.Error(w, "followup failed", 500)
		case "/api/v1/terraform/team/read/t-1":
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveTeam().Schema, map[string]interface{}{"name": "ops", "slack_channel_id": "C123"})
	diags := ResourceAdaptiveTeamCreate(context.Background(), d, client)
	if diags.HasError() || len(diags) == 0 || d.Id() != "t-1" {
		t.Fatal("eventual-consistency read lost created group identity", diags, d.Id())
	}
}

func TestScheduleNullCapabilitiesRejectedAtomically(t *testing.T) {
	for _, key := range []string{"isActive", "allDay", "users", "teams", "endpoints"} {
		t.Run(key, func(t *testing.T) {
			body := map[string]interface{}{"id": "s-1", "name": "changed", "scheduleType": "weekdays", "isActive": true, "allDay": false, "users": []string{}, "teams": []string{}, "endpoints": []string{}, "startHour": 0, "startMinute": 0, "endHour": 0, "endMinute": 0}
			body[key] = nil
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			client := scheduleReadServer(t, string(raw))
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "start_hour": 9, "is_active": true, "users": []interface{}{"member@example.com"}})
			d.SetId("s-1")
			before := identityStateSnapshot(d)
			if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); !diags.HasError() {
				t.Fatal("null capability adopted as known false/empty")
			}
			if !reflect.DeepEqual(before, identityStateSnapshot(d)) {
				t.Fatal("null capability mutated state")
			}
		})
	}
}

func TestScheduleNullWindowRejectedAtomically(t *testing.T) {
	client := scheduleReadServer(t, `{"id":"s-1","name":"changed","scheduleType":"weekdays","isActive":true,"allDay":false,"users":[],"teams":[],"endpoints":[],"startHour":null,"startMinute":0,"endHour":0,"endMinute":0}`)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "start_hour": 9, "end_hour": 17})
	d.SetId("s-1")
	before := identityStateSnapshot(d)
	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); !diags.HasError() {
		t.Fatal("null window cannot establish a known zero")
	}
	if !reflect.DeepEqual(before, identityStateSnapshot(d)) {
		t.Fatal("null window mutated state")
	}
}

func TestScheduleUnresolvedReadPreservesFullState(t *testing.T) {
	client := scheduleReadServer(t, `{"id":"s-1","name":"changed","scheduleType":"weekdays","users":[],"teams":[],"endpoints":[],"startHour":0,"startMinute":0,"endHour":0,"endMinute":0,"unresolvedUsers":["deleted-user-id"]}`)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "users": []interface{}{"a@example.com"}})
	d.SetId("s-1")
	before := identityStateSnapshot(d)
	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); !diags.HasError() {
		t.Fatal("incomplete name resolution must not be adopted as final-user removal")
	}
	if !reflect.DeepEqual(before, identityStateSnapshot(d)) {
		t.Fatal("unresolved read mutated state")
	}
}

func TestScriptMetadataRemovalBuildsExplicitEmptyMap(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveScript().Schema, map[string]interface{}{"name": "job", "endpoint": "db", "command": "echo test"})
	if scriptRequestFromSchema(d).ParameterDescriptions == nil {
		t.Fatal("nil metadata preserves backend map instead of clearing")
	}
}

func TestFamiliesCreateRejectBlankRequiredBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		r      *schema.Resource
		values map[string]interface{}
	}{
		{"authorization", ResourceAdaptiveAuthorization(), map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": " "}},
		{"group", ResourceAdaptiveTeam(), map[string]interface{}{"name": "ops", "members": []interface{}{" "}}},
		{"script", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "command": " ", "endpoint": "db"}},
		{"schedule", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays", "users": []interface{}{" "}}},
		{"dataprotection", ResourceAdaptiveDataProtection(), map[string]interface{}{"resource": " "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, tc.r.Schema, tc.values)
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("invalid input reached HTTP")
				http.Error(w, "blocked", 500)
			})
			diags := tc.r.CreateContext(context.Background(), d, client)
			if !diags.HasError() || len(api.paths) != 0 {
				t.Fatal("create guard failed", diags, api.paths)
			}
		})
	}
}

func TestAuthorizationMongo36UsesBackendSpelling(t *testing.T) {
	if diags := validateAuthorizedResourceType("mongodb36", nil); diags.HasError() {
		t.Fatal("backend model.Mongo36 spelling rejected", diags)
	}
	if diags := validateAuthorizedResourceType("mongo36", nil); !diags.HasError() {
		t.Fatal("invented mongo36 spelling accepted")
	}
}

func TestFamiliesApplyGuardsRejectUnsupportedClearsBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name            string
		r               *schema.Resource
		values          map[string]interface{}
		field, old, new string
	}{
		{"script", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "endpoint": "db", "command": "echo test", "description": "old"}, "description", "old", ""},
		{"authorization", ResourceAdaptiveAuthorization(), map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": "SELECT", "description": "old"}, "description", "old", ""},
		{"schedule", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays", "max_access_time": 60}, "max_access_time", "60", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := schema.TestResourceDataRaw(t, tc.r.Schema, tc.values)
			original.SetId("existing")
			d, err := schema.InternalMap(tc.r.Schema).Data(original.State(), &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{tc.field: {Old: tc.old, New: tc.new}}})
			if err != nil {
				t.Fatal(err)
			}
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("unsafe mutation attempted")
				http.Error(w, "blocked", 500)
			})
			diags := tc.r.UpdateContext(context.Background(), d, client)
			if !diags.HasError() || len(api.paths) != 0 || d.Id() != "existing" {
				t.Fatal("apply guard failed", diags, api.paths, d.Id())
			}
		})
	}
}

func TestScheduleLegacyReadPreservesAbsentCapabilities(t *testing.T) {
	client := scheduleReadServer(t, `{"id":"s-1","name":"window","scheduleType":"weekdays","mappedEndpoints":["endpoint-uuid"]}`)
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{"name": "window", "schedule_type": "weekdays", "all_day": true, "is_active": true, "endpoints": []interface{}{"db"}})
	d.SetId("s-1")
	diags := ResourceAdaptiveScheduleRead(context.Background(), d, client)
	if diags.HasError() || len(diags) == 0 {
		t.Fatal("legacy capability loss must warn, not invent full read support", diags)
	}
	if !d.Get("all_day").(bool) || !d.Get("is_active").(bool) {
		t.Fatal("absent flags overwritten")
	}
	if got := stringList(d, "endpoints"); len(got) != 1 || got[0] != "db" {
		t.Fatal("legacy endpoint IDs treated as canonical names")
	}
}

func TestFamiliesRejectBlankInputsAtPlan(t *testing.T) {
	for _, tc := range []struct {
		name   string
		r      *schema.Resource
		values map[string]interface{}
		fields []string
	}{
		{"authorization", ResourceAdaptiveAuthorization(), map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": "SELECT"}, []string{"name", "resource_type", "permissions"}},
		{"group", ResourceAdaptiveTeam(), map[string]interface{}{"name": "ops", "members": []interface{}{"a@example.com"}, "endpoints": []interface{}{"db"}}, []string{"name", "members", "endpoints"}},
		{"script", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "endpoint": "db", "command": "echo test"}, []string{"name", "endpoint", "command"}},
		{"schedule", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays", "users": []interface{}{"a@example.com"}, "teams": []interface{}{"ops"}, "endpoints": []interface{}{"db"}}, []string{"name", "schedule_type", "users", "teams", "endpoints"}},
		{"dataprotection", ResourceAdaptiveDataProtection(), map[string]interface{}{"resource": "db"}, []string{"resource"}},
	} {
		for _, field := range tc.fields {
			t.Run(tc.name+"/"+field, func(t *testing.T) {
				values := map[string]interface{}{}
				for k, v := range tc.values {
					values[k] = v
				}
				if _, ok := values[field].([]interface{}); ok {
					values[field] = []interface{}{" "}
				} else {
					values[field] = " "
				}
				if _, err := tc.r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(values), nil); err == nil {
					t.Fatal("blank input accepted at plan")
				}
			})
		}
	}
}

func TestScriptCreateDigestCaptureFailureRetainsIdentityWithWarning(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/terraform/script/create":
			w.Write([]byte(`{"id":"sc-1"}`))
		case "/api/v1/terraform/script/read/sc-1":
			http.Error(w, "read unavailable", 500)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveScript().Schema, map[string]interface{}{"name": "job", "command": "echo test", "endpoint": "db"})
	diags := ResourceAdaptiveScriptCreate(context.Background(), d, client)
	if diags.HasError() || len(diags) == 0 || d.Id() != "sc-1" {
		t.Fatal("post-create capture failure must retain identity and warn without taint", diags, d.Id())
	}
}

func TestFamilyUnsupportedClearsRejectedAtPlan(t *testing.T) {
	for _, tc := range []struct {
		name   string
		r      *schema.Resource
		values map[string]interface{}
		field  string
	}{
		{"script description", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "command": "echo test", "endpoint": "db", "description": "old"}, "description"},
		{"authorization description", ResourceAdaptiveAuthorization(), map[string]interface{}{"name": "role", "resource_type": "mysql", "permissions": "SELECT", "description": "old"}, "description"},
		{"schedule nullable duration", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays", "max_access_time": 60}, "max_access_time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, tc.r.Schema, tc.values)
			d.SetId("existing")
			delete(tc.values, tc.field)
			if _, err := tc.r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(tc.values), nil); err == nil {
				t.Fatal("unsupported backend clear accepted")
			}
		})
	}
}

func TestTeamSlackFollowupFailureRemainsRetryable(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/terraform/team/create":
			w.Write([]byte(`{"id":"t-1"}`))
		case "/api/v1/terraform/team/update/t-1":
			http.Error(w, "followup failed", 500)
		case "/api/v1/terraform/team/read/t-1":
			w.Write([]byte(`{"id":"t-1","name":"oncall","members":[],"endpoints":[],"slackChannelId":""}`))
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveTeam().Schema, map[string]interface{}{"name": "oncall", "slack_channel_id": "C123"})
	diags := ResourceAdaptiveTeamCreate(context.Background(), d, client)
	if diags.HasError() {
		t.Fatal("post-create failure would taint and duplicate an existing group", diags)
	}
	if len(diags) == 0 || d.Id() != "t-1" {
		t.Fatal("must warn and retain trackable identity")
	}
	if d.Get("slack_channel_id").(string) != "" {
		t.Fatal("remote omission not recorded for next update retry")
	}
}

func TestScriptReadRetainsLastWriteDigestWhenMetadataWithheld(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(scriptReadBody(""))) })
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveScript().Schema, map[string]interface{}{"name": "backup", "endpoint": "ep1", "command": "original", "command_digest": "written"})
	d.SetId("sc-1")
	for i := 0; i < 2; i++ {
		if diags := ResourceAdaptiveScriptRead(context.Background(), d, client); diags.HasError() {
			t.Fatal(diags)
		}
	}
	if d.Get("command_digest").(string) != "written" {
		t.Fatal("missing metadata erased last-write evidence")
	}
}

func TestFamilyUnsupportedUpdatesRejectedAtPlan(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resource *schema.Resource
		values   map[string]interface{}
		field    string
		next     interface{}
	}{
		{"script", ResourceAdaptiveScript(), map[string]interface{}{"name": "job", "endpoint": "db", "command": "echo test"}, "endpoint", "other"},
		{"dataprotection", ResourceAdaptiveDataProtection(), map[string]interface{}{"resource": "db"}, "resource", "other"},
		{"schedule timezone", ResourceAdaptiveSchedule(), map[string]interface{}{"name": "window", "schedule_type": "weekdays", "timezone": "UTC"}, "timezone", "Asia/Kolkata"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, tc.resource.Schema, tc.values)
			d.SetId("existing")
			tc.values[tc.field] = tc.next
			if _, err := tc.resource.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(tc.values), nil); err == nil {
				t.Fatal("unsupported update accepted at plan")
			}
			if tc.resource.Schema[tc.field].ForceNew {
				t.Fatal("unsafe replacement instead of plan rejection")
			}
		})
	}
}

func TestScheduleModernReadClearsFinalValues(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSchedule().Schema, map[string]interface{}{
		"name": "window", "schedule_type": "custom", "description": "old", "expires_at": "2030-01-01T00:00:00Z",
		"timezone": "UTC", "start_hour": 9, "end_hour": 17, "start_day": 1, "end_day": 2, "max_access_time": 60,
		"weekdays": []interface{}{"Monday"}, "specific_dates": []interface{}{"2030-01-01T00:00:00Z"},
		"users": []interface{}{"a@example.com"}, "teams": []interface{}{"ops"}, "endpoints": []interface{}{"db"},
	})
	d.SetId("s-1")
	// Independent wire fixture: current backend omits empty scalar/pattern values,
	// but always emits the four window integers and all three canonical lists.
	client := scheduleReadServer(t, `{"id":"s-1","name":"window","scheduleType":"custom","isActive":false,"allDay":false,"startHour":0,"startMinute":0,"endHour":0,"endMinute":0,"users":[],"teams":[],"endpoints":[],"mappedEndpoints":["legacy-id"]}`)
	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatal(diags)
	}
	for _, key := range []string{"description", "expires_at", "timezone"} {
		if d.Get(key).(string) != "" {
			t.Errorf("%s not cleared", key)
		}
	}
	for _, key := range []string{"start_hour", "end_hour", "start_day", "end_day", "max_access_time"} {
		if d.Get(key).(int) != 0 {
			t.Errorf("%s not reset", key)
		}
	}
	for _, key := range []string{"weekdays", "specific_dates", "users", "teams", "endpoints"} {
		if len(stringList(d, key)) != 0 {
			t.Errorf("%s not removed", key)
		}
	}
	if d.Get("is_active").(bool) {
		t.Error("active not reset")
	}
}
