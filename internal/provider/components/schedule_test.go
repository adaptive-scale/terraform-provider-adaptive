package components

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// scheduleRequestFromSchema must carry operation_type through to the API request
// so a Terraform-declared autoreject schedule actually reaches the backend.
func TestScheduleRequestFromSchema_OperationType(t *testing.T) {
	res := ResourceAdaptiveSchedule()

	for _, want := range []string{"autoreject", "autoapprove"} {
		d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
			"name":           "op-" + want,
			"schedule_type":  "weekdays",
			"all_day":        true,
			"operation_type": want,
		})
		req, err := scheduleRequestFromSchema(d)
		if err != nil {
			t.Fatalf("scheduleRequestFromSchema(%s): %v", want, err)
		}
		if req.OperationType != want {
			t.Errorf("operation_type not mapped: got %q want %q", req.OperationType, want)
		}
	}
}

// ResourceAdaptiveScheduleRead must refresh expires_at from the backend's read
// response into Terraform state. Without this the attribute never round-trips and
// every plan shows a spurious diff on expires_at. Drives the real GetSchedule +
// Read against a fake backend that returns the expiresAt the backend would emit
// (RFC3339 UTC).
func TestResourceAdaptiveScheduleRead_RefreshesExpiresAt(t *testing.T) {
	const want = "2030-12-31T23:59:59Z"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/terraform/schedule/read/") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "sch-1",
			"name": "expiry-test",
			"scheduleType": "weekdays",
			"isActive": true,
			"allDay": true,
			"expiresAt": "` + want + `"
		}`))
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveSchedule()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":          "expiry-test",
		"schedule_type": "weekdays",
		"all_day":       true,
	})
	d.SetId("sch-1")

	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	if got := d.Get("expires_at").(string); got != want {
		t.Errorf("expires_at not refreshed from read response: got %q want %q", got, want)
	}
}

// scheduleReadServer serves one canned read response.
func scheduleReadServer(t *testing.T, body string) *adaptive.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/terraform/schedule/read/") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return adaptive.NewClient("test-token", srv.URL)
}

// The read now covers the pattern and membership fields, so genuine drift in
// them reaches state.
func TestResourceAdaptiveScheduleRead_RefreshesFullView(t *testing.T) {
	client := scheduleReadServer(t, `{
		"id": "sch-1", "name": "biz-hours", "scheduleType": "custom", "isActive": true,
		"allDay": false, "description": "business hours",
		"startHour": 9, "startMinute": 30, "endHour": 17, "endMinute": 0,
		"weekdays": ["monday", "friday"], "startDay": 1, "endDay": 15,
		"users": ["u@x.com"], "teams": ["oncall"], "endpoints": ["ep2"],
		"maxAccessTime": 60
	}`)

	res := ResourceAdaptiveSchedule()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":          "biz-hours",
		"schedule_type": "custom",
		"start_hour":    8,
		"weekdays":      []interface{}{"Monday"},
		"endpoints":     []interface{}{"ep1"},
	})
	d.SetId("sch-1")

	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}
	if got := d.Get("description").(string); got != "business hours" {
		t.Errorf("description: got %q", got)
	}
	if got := d.Get("start_hour").(int); got != 9 {
		t.Errorf("start_hour: got %d", got)
	}
	if got := d.Get("start_minute").(int); got != 30 {
		t.Errorf("start_minute: got %d", got)
	}
	if got := d.Get("end_hour").(int); got != 17 {
		t.Errorf("end_hour: got %d", got)
	}
	if got := d.Get("start_day").(int); got != 1 {
		t.Errorf("start_day: got %d", got)
	}
	if got := d.Get("max_access_time").(int); got != 60 {
		t.Errorf("max_access_time: got %d", got)
	}
	if got := stringList(d, "endpoints"); len(got) != 1 || got[0] != "ep2" {
		t.Errorf("endpoints should follow the server, got %v", got)
	}
	if got := stringList(d, "users"); len(got) != 1 || got[0] != "u@x.com" {
		t.Errorf("users: got %v", got)
	}
	// The server lowercases weekday names — that alone is not drift, but the
	// added Friday is, so the server list wins here.
	if got := stringList(d, "weekdays"); len(got) != 2 {
		t.Errorf("weekdays should be refreshed when membership changed, got %v", got)
	}
}

// The server lowercases weekday names and normalizes timestamps to UTC.
// Rewriting state with those spellings would show a diff on every plan.
func TestResourceAdaptiveScheduleRead_KeepsEquivalentSpellings(t *testing.T) {
	client := scheduleReadServer(t, `{
		"id": "sch-1", "name": "dates", "scheduleType": "specific", "isActive": true, "allDay": true,
		"weekdays": ["monday", "tuesday"],
		"specificDates": ["2030-01-01T00:00:00Z"]
	}`)

	res := ResourceAdaptiveSchedule()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":           "dates",
		"schedule_type":  "specific",
		"all_day":        true,
		"weekdays":       []interface{}{"Monday", "Tuesday"},
		"specific_dates": []interface{}{"2030-01-01T05:30:00+05:30"},
	})
	d.SetId("sch-1")

	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}
	if got := stringList(d, "weekdays"); got[0] != "Monday" {
		t.Errorf("a case-only difference must not rewrite weekdays, got %v", got)
	}
	if got := stringList(d, "specific_dates"); got[0] != "2030-01-01T05:30:00+05:30" {
		t.Errorf("the same instant must keep the configured spelling, got %v", got)
	}
}

// Older backends answer with a lossy view. Those omissions decode as zero
// values and must not overwrite a configured window or membership.
func TestResourceAdaptiveScheduleRead_LossyResponseKeepsState(t *testing.T) {
	client := scheduleReadServer(t, `{
		"id": "sch-1", "name": "biz-hours", "scheduleType": "custom",
		"isActive": true, "allDay": false
	}`)

	res := ResourceAdaptiveSchedule()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":            "biz-hours",
		"schedule_type":   "custom",
		"description":     "business hours",
		"start_hour":      9,
		"end_hour":        17,
		"weekdays":        []interface{}{"Monday"},
		"users":           []interface{}{"u@x.com"},
		"max_access_time": 60,
	})
	d.SetId("sch-1")

	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}
	if got := d.Get("start_hour").(int); got != 9 {
		t.Errorf("start_hour was clobbered by an omitted field: got %d", got)
	}
	if got := d.Get("description").(string); got != "business hours" {
		t.Errorf("description was clobbered: got %q", got)
	}
	if got := d.Get("max_access_time").(int); got != 60 {
		t.Errorf("max_access_time was clobbered: got %d", got)
	}
	if got := stringList(d, "weekdays"); len(got) != 1 {
		t.Errorf("weekdays were clobbered: got %v", got)
	}
	if got := stringList(d, "users"); len(got) != 1 {
		t.Errorf("users were clobbered: got %v", got)
	}
}

func TestResourceAdaptiveScheduleRead_DropsDeleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveSchedule()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":          "gone",
		"schedule_type": "weekdays",
	})
	d.SetId("sch-1")

	if diags := ResourceAdaptiveScheduleRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("a deleted schedule must not fail the refresh: %+v", diags)
	}
	if d.Id() != "" {
		t.Errorf("id should be cleared, got %q", d.Id())
	}
}
