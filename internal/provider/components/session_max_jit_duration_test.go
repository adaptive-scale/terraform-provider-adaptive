package components

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"net/http"
	"testing"
)

func TestMaxJITDurationSchema(t *testing.T) {
	s := ResourceAdaptiveSession().Schema["max_jit_request_duration"]
	if s == nil {
		t.Fatal("missing max_jit_request_duration schema")
	}
	if s.Type != schema.TypeString || !s.Optional || s.Computed || s.Required || s.Default != nil || s.DefaultFunc != nil {
		t.Fatalf("duration must be an optional string without a default: %+v", s)
	}
}

// Build ResourceData through the public state API so raw config presence is real.
func maxJITDurationData(raw cty.Value, prior string, isImport bool) *schema.ResourceData {
	attrs := map[string]string{"max_jit_request_duration": prior}
	if !isImport {
		attrs["name"] = "endpoint"
		attrs["resource"] = "database"
	}
	return ResourceAdaptiveSession().Data(&terraform.InstanceState{ID: "ep-1", Attributes: attrs, RawConfig: raw})
}

func TestMaxJITDurationBuilderPresence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  cty.Value
		prior   string
		present bool
	}{
		{"omitted", cty.NullVal(cty.String), "", false},
		{"omitted with prior", cty.NullVal(cty.String), "2h", false},
		{"explicit empty", cty.StringVal(""), "", true},
		{"explicit value", cty.StringVal("2h"), "2h", true},
		{"unknown", cty.UnknownVal(cty.String), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := maxJITDurationData(cty.ObjectVal(map[string]cty.Value{"max_jit_request_duration": tc.config}), tc.prior, false)
			req, err := sessionRequestFromSchema(d)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			got, present := body["max_jit_request_duration"]
			if present != tc.present || (present && got != tc.prior) {
				t.Fatalf("got %#v present=%v, want %q present=%v", got, present, tc.prior, tc.present)
			}
		})
	}
}

func TestMaxJITDurationReadOwnership(t *testing.T) {
	for _, tc := range []struct {
		name                string
		raw                 cty.Value
		prior, server, want string
		isImport            bool
	}{
		{"omitted config", cty.ObjectVal(map[string]cty.Value{"max_jit_request_duration": cty.NullVal(cty.String)}), "", "2h", "", false},
		{"configured drift", cty.ObjectVal(map[string]cty.Value{"max_jit_request_duration": cty.StringVal("1h")}), "1h", "2h", "2h", false},
		{"configured empty catches drift", cty.ObjectVal(map[string]cty.Value{"max_jit_request_duration": cty.StringVal("")}), "", "2h", "2h", false},
		{"configured clear remains empty", cty.ObjectVal(map[string]cty.Value{"max_jit_request_duration": cty.StringVal("")}), "", "", "", false},
		{"refresh omitted", cty.NilVal, "", "2h", "", false},
		{"refresh owned drift", cty.NilVal, "1h", "2h", "2h", false},
		{"refresh cleared override", cty.NilVal, "1h", "", "", false},
		{"refresh empty avoids diff", cty.NilVal, "", "2h", "", false},
		{"import recovers override", cty.NilVal, "", "2h", "2h", true},
		{"import inherited", cty.NilVal, "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"id":"ep-1","name":"endpoint","resource":"database","sessionType":"cli","maxJitRequestDuration":%q}`, tc.server)
			})
			d := maxJITDurationData(tc.raw, tc.prior, tc.isImport)
			if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
				t.Fatalf("read: %+v", diags)
			}
			if got := d.Get("max_jit_request_duration"); got != tc.want {
				t.Fatalf("got %#v, want %q", got, tc.want)
			}
			// An imported override remains refreshable after the import-only read.
			if tc.isImport && tc.want != "" {
				if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
					t.Fatalf("second read: %+v", diags)
				}
				if got := d.Get("max_jit_request_duration"); got != tc.want {
					t.Fatalf("second read: got %#v, want %q", got, tc.want)
				}
			}
		})
	}
}
