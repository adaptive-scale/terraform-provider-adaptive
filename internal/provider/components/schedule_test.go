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
