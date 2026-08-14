package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The endpoint create request must carry dlp_policy through to the backend —
// otherwise a Terraform-declared DLP policy attachment never reaches the
// server. Drives the real client against a fake backend and inspects the
// session create body.
func TestResourceAdaptiveSessionCreate_CarriesDLPPolicy(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/terraform/session/create":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "sess-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/terraform/session/read/sess-1":
			w.Header().Set("Content-Type", "application/json")
			// The read poller expects 202 and a terminal status.
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"Status": "created"}`))
		default:
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveSession()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":       "pg-endpoint",
		"resource":   "pg-resource",
		"dlp_policy": "pii-strict",
	})

	if diags := ResourceAdaptiveSessionCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create returned diagnostics: %+v", diags)
	}

	if got := gotBody["dlp_policy"]; got != "pii-strict" {
		t.Errorf("dlp_policy not carried in session create request: got %v want %q", got, "pii-strict")
	}
}

// When no dlp_policy is configured the field must be omitted from the request
// body entirely (json omitempty), so older backends see no behavior change.
func TestResourceAdaptiveSessionCreate_OmitsEmptyDLPPolicy(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/terraform/session/create":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "sess-2"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/terraform/session/read/sess-2":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"Status": "created"}`))
		default:
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveSession()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":     "pg-endpoint",
		"resource": "pg-resource",
	})

	if diags := ResourceAdaptiveSessionCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create returned diagnostics: %+v", diags)
	}

	if _, present := gotBody["dlp_policy"]; present {
		t.Errorf("dlp_policy should be omitted when unset, request body carried %v", gotBody["dlp_policy"])
	}
}
