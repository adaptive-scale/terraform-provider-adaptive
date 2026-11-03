package components

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func endpointContractData(t *testing.T, values map[string]interface{}, raw cty.Value) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, values)
	d.SetId("ep-1")
	state := d.State()
	state.RawConfig = raw
	return ResourceAdaptiveSession().Data(state)
}

func TestEndpointJITModeUnmanagedPresence(t *testing.T) {
	s := ResourceAdaptiveSession().Schema["jit_mode"]
	if !s.Optional || !s.Computed || s.Default != nil {
		t.Error("jit_mode must observe when unmanaged")
	}
	for _, tc := range []struct {
		name string
		raw  cty.Value
		want string
	}{
		{"no raw config", cty.NilVal, ""},
		{"absent", cty.EmptyObjectVal, ""},
		{"null", cty.ObjectVal(map[string]cty.Value{"jit_mode": cty.NullVal(cty.String)}), ""},
		{"unknown", cty.ObjectVal(map[string]cty.Value{"jit_mode": cty.UnknownVal(cty.String)}), ""},
		{"empty", cty.ObjectVal(map[string]cty.Value{"jit_mode": cty.StringVal("")}), ""},
		{"configured", cty.ObjectVal(map[string]cty.Value{"jit_mode": cty.StringVal("both")}), "both"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := endpointContractData(t, map[string]interface{}{"name": "endpoint", "resource": "database", "jit_mode": "both"}, tc.raw)
			req, err := sessionRequestFromSchema(d)
			if err != nil {
				t.Fatal(err)
			}
			if req.JITMode != tc.want {
				t.Fatalf("request mode %q, want %q; state must not infer ownership", req.JITMode, tc.want)
			}
		})
	}
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(adaptive.SessionReadResponse{ID: "ep-1", Name: "endpoint", Resource: "database", SessionType: "cli", JITMode: "script"})
	})
	d := endpointContractData(t, map[string]interface{}{"name": "endpoint", "resource": "database"}, cty.NilVal)
	if ds := ResourceAdaptiveSessionRead(context.Background(), d, client); ds.HasError() {
		t.Fatal(ds)
	}
	if d.Get("jit_mode") != "script" {
		t.Error("unmanaged mode was not observed")
	}
	for _, configured := range []string{"", "script", "both"} {
		diff, err := ResourceAdaptiveSession().Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(map[string]interface{}{"name": "endpoint", "resource": "database", "jit_mode": configured}), nil)
		if err != nil {
			t.Fatal(err)
		}
		changed := diff != nil && diff.Attributes["jit_mode"] != nil
		if changed != (configured == "both") {
			t.Fatalf("mode %q changed=%v: empty means unmanaged, not clear", configured, changed)
		}
	}
}

func TestEndpointTypePlanContract(t *testing.T) {
	r := ResourceAdaptiveSession()
	if r.Schema["type"].ForceNew {
		t.Fatal("type changes must be rejected, not destructively replaced")
	}
	if _, valid := getSessionType("services"); valid {
		t.Error("unsupported services type must never be forwarded to the backend")
	}
	for _, prior := range []string{"", "direct", "cli", "client"} {
		for _, next := range []string{"", "direct", "cli", "client", "services", "bogus"} {
			t.Run(prior+"/"+next, func(t *testing.T) {
				d := endpointContractData(t, map[string]interface{}{"name": "endpoint", "resource": "database", "type": prior}, cty.NilVal)
				diff, err := r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(map[string]interface{}{"name": "endpoint", "resource": "database", "type": next}), nil)
				oldWire, _ := getSessionType(prior)
				newWire, valid := getSessionType(next)
				if (err != nil) != (!valid || oldWire != newWire) {
					t.Fatalf("plan transition %s -> %s: %v", prior, next, err)
				}
				if valid && oldWire == newWire && diff != nil && diff.Attributes["type"] != nil {
					t.Error("equivalent type aliases must not plan an update")
				}
			})
		}
	}
	_, err := r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "endpoint", "resource": "database", "type": "bogus"}), nil)
	if err == nil {
		t.Error("invalid create type accepted at plan")
	}
	_, err = r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "endpoint", "resource": "database", "type": ""}), nil)
	if err != nil {
		t.Fatalf("empty type must remain a cli alias: %v", err)
	}
	// SDK hcl2shim.UnknownVariableValue (internal, so not importable here).
	_, err = r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(map[string]interface{}{"name": "endpoint", "resource": "database", "type": "74D93920-ED26-11E3-AC10-0800200C9A66"}), nil)
	if err != nil {
		t.Fatalf("unknown type must defer validation: %v", err)
	}
}

func TestEndpointTypeSavedPlanGuard(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"id":"ep-1","type":"jit"}`)) })
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{"name": "endpoint", "resource": "database", "type": "client"})
	d.SetId("ep-1")
	// GetChange sees the original type despite the applied new value.
	if err := d.Set("type", "direct"); err != nil {
		t.Fatal(err)
	}
	ds := ResourceAdaptiveSessionUpdate(context.Background(), d, client)
	if !ds.HasError() {
		t.Fatal("saved plan type transition accepted")
	}
	if !strings.Contains(ds[0].Summary, "type") {
		t.Fatalf("unsafe/unhelpful diagnostic: %+v", ds)
	}
	if len(api.paths) != 0 {
		t.Fatalf("type transition reached HTTP: %v", api.paths)
	}
	d = endpointContractData(t, map[string]interface{}{"name": "endpoint", "resource": "database", "type": "services"}, cty.NilVal)
	if _, err := sessionRequestFromSchema(d); err == nil {
		t.Error("unsupported services type accepted by request builder")
	}
	if ds := ResourceAdaptiveSessionCreate(context.Background(), d, client); !ds.HasError() {
		t.Error("unsupported services type accepted by create callback")
	}
	if ds := ResourceAdaptiveSessionUpdate(context.Background(), d, client); !ds.HasError() {
		t.Error("unsupported services type accepted by update callback")
	}
	if len(api.paths) != 0 {
		t.Error("unsupported type reached HTTP")
	}
}

func TestEndpointPolicyOmissionAfterImportPreservesFullReplace(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "ep-1", "name": "endpoint", "resource": "database", "sessionType": "cli", "isJitEnabled": true, "scriptOnlyAccess": true, "disableOutputCapture": true, "disableDataStudio": true, "disableWebCli": true, "autoApproval": true, "jitMultiApprover": true, "jitTotalApprovers": 3, "maxJitRequestDuration": "2h", "accessApprovers": []string{"approver@example.com"}})
	})
	d := ResourceAdaptiveSession().Data(&terraform.InstanceState{ID: "ep-1"})
	if ds := ResourceAdaptiveSessionRead(context.Background(), d, client); ds.HasError() {
		t.Fatal(ds)
	}
	if err := d.Set("tags", []string{"env=test"}); err != nil {
		t.Fatal(err)
	}
	req, err := sessionRequestFromSchema(d)
	if err != nil {
		t.Fatal(err)
	}
	if !req.IsJITEnabled || !req.ScriptOnlyAccess || !req.DisableOutputCapture || !req.DisableDataStudio || !req.DisableWebCLI || len(req.AccessApprovers) != 1 {
		t.Fatalf("tag-only request lost observed full-replace policy: %+v", req)
	}
	if req.AutoApproval != nil || req.JITMultiApprover != nil || req.JITTotalApprovers != nil || req.MaxJITRequestDuration != nil {
		t.Fatal("observed pointer fields acquired ownership")
	}
	// Omitting policy in a real SDK plan must retain observed state, not plan
	// resets. Explicit zero values, in contrast, produce corrective changes.
	base := map[string]interface{}{"name": "endpoint", "resource": "database", "tags": []interface{}{"env=test"}}
	diff, err := ResourceAdaptiveSession().Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"is_jit_enabled", "script_only_access", "disable_output_capture", "disable_data_studio", "disable_web_cli", "jit_approvers", "auto_approval", "jit_multi_approver", "jit_total_approvers", "max_jit_request_duration"} {
		if diff != nil && (diff.Attributes[key] != nil || diff.Attributes[key+".#"] != nil) {
			t.Errorf("omission planned policy change: %s", key)
		}
	}
	clears := map[string]interface{}{"is_jit_enabled": false, "script_only_access": false, "disable_output_capture": false, "disable_data_studio": false, "disable_web_cli": false, "jit_approvers": []interface{}{}, "auto_approval": false, "jit_multi_approver": false, "jit_total_approvers": 0, "max_jit_request_duration": ""}
	raw := map[string]cty.Value{}
	for key, value := range clears {
		base[key] = value
		switch v := value.(type) {
		case bool:
			raw[key] = cty.BoolVal(v)
		case int:
			raw[key] = cty.NumberIntVal(int64(v))
		case string:
			raw[key] = cty.StringVal(v)
		default:
			raw[key] = cty.ListValEmpty(cty.String)
		}
	}
	state := d.State()
	// The SDK's public Diff copies protocol RawConfig from InstanceState.
	state.RawConfig = cty.ObjectVal(raw)
	diff, err = ResourceAdaptiveSession().Diff(context.Background(), state, terraform.NewResourceConfigRaw(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	for key := range clears {
		if diff == nil || (diff.Attributes[key] == nil && diff.Attributes[key+".#"] == nil) {
			t.Errorf("explicit clear did not plan ownership: %s", key)
		}
	}
	cleared := endpointContractData(t, base, cty.ObjectVal(raw))
	req, err = sessionRequestFromSchema(cleared)
	if err != nil {
		t.Fatal(err)
	}
	if req.IsJITEnabled || req.ScriptOnlyAccess || req.DisableOutputCapture || req.DisableDataStudio || req.DisableWebCLI || len(req.AccessApprovers) != 0 {
		t.Fatal("explicit full-replace clears were lost")
	}
	if req.AutoApproval == nil || *req.AutoApproval || req.JITMultiApprover == nil || *req.JITMultiApprover || req.JITTotalApprovers == nil || *req.JITTotalApprovers != 0 || req.MaxJITRequestDuration == nil || *req.MaxJITRequestDuration != "" {
		t.Fatal("explicit pointer clears were omitted")
	}
}
