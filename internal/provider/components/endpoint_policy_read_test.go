package components

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func completeEndpointPolicyFrame() map[string]interface{} {
	return map[string]interface{}{
		"id": "ep-1", "name": "endpoint", "resource": "database", "sessionType": "cli",
		"isJitEnabled": false, "scriptOnlyAccess": false, "disableOutputCapture": false,
		"disableDataStudio": false, "disableWebCli": false, "accessApprovers": []string{},
	}
}

func TestEndpointIncompletePolicyReadDoesNotMutateState(t *testing.T) {
	for _, key := range []string{"resource", "sessionType", "isJitEnabled", "scriptOnlyAccess", "disableOutputCapture", "disableDataStudio", "disableWebCli", "accessApprovers"} {
		t.Run(key, func(t *testing.T) {
			frame := completeEndpointPolicyFrame()
			delete(frame, key)
			frame["name"] = "must-not-replace-name"
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(frame) })
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{
				"name": "endpoint", "resource": "database", "is_jit_enabled": true,
			})
			d.SetId("ep-1")
			before := d.State()
			if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); !diags.HasError() {
				t.Error("incomplete endpoint policy read was accepted as known zero values")
			}
			if !reflect.DeepEqual(before, d.State()) {
				t.Error("incomplete endpoint read partially mutated tracked state")
			}
		})
	}
}

func TestEndpointUpdateRejectsIncompletePolicyBeforeMutation(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		frame := completeEndpointPolicyFrame()
		delete(frame, "isJitEnabled")
		_ = json.NewEncoder(w).Encode(frame)
	})
	d := maxJITDurationData(cty.ObjectVal(map[string]cty.Value{}), "", false)
	if diags := ResourceAdaptiveSessionUpdate(context.Background(), d, client); !diags.HasError() {
		t.Error("unknown policy values allowed an endpoint update")
	}
	if api.sawPath("/update/") {
		t.Error("incomplete preflight policy reached mutation endpoint")
	}
}

func TestEndpointUpdateRetainsOriginalIdentityOnMismatchedResponse(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_, _ = w.Write([]byte(`{"id":"different-endpoint"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(completeEndpointPolicyFrame())
	})
	d := maxJITDurationData(cty.ObjectVal(map[string]cty.Value{}), "", false)
	if diags := ResourceAdaptiveSessionUpdate(context.Background(), d, client); !diags.HasError() {
		t.Error("mismatched update identity accepted")
	}
	if d.Id() != "ep-1" || api.sawPath("/read/different-endpoint") {
		t.Error("update followed a response ID instead of preserving the exact tracked endpoint")
	}
}

func TestEndpointUpdateRejectsUnconfirmedPolicyWrite(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_, _ = w.Write([]byte(`{"id":"ep-1"}`))
			return
		}
		frame := completeEndpointPolicyFrame()
		frame["autoApproval"] = true // A 200 update silently ignored requested false.
		_ = json.NewEncoder(w).Encode(frame)
	})
	d := maxJITDurationData(cty.ObjectVal(map[string]cty.Value{"auto_approval": cty.BoolVal(false)}), "", false)
	before := d.State()
	if diags := ResourceAdaptiveSessionUpdate(context.Background(), d, client); !diags.HasError() {
		t.Error("successful HTTP response was mistaken for confirmed policy persistence")
	}
	if !reflect.DeepEqual(before, d.State()) {
		t.Error("unconfirmed policy write mutated tracked state")
	}
}

func TestEndpointMissingOptionalPolicyRetainsPriorObservation(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(completeEndpointPolicyFrame())
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{
		"name": "endpoint", "resource": "database", "auto_approval": true, "jit_multi_approver": true, "jit_total_approvers": 3,
	})
	d.SetId("ep-1")
	if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
		t.Fatal("complete replacement-policy frame rejected")
	}
	if d.Get("auto_approval") != true || d.Get("jit_multi_approver") != true || d.Get("jit_total_approvers") != 3 {
		t.Error("missing optional read fields were treated as known false/zero")
	}
}
