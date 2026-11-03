package components

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestEndpointRefreshObservesBothSecurityDirections(t *testing.T) {
	for attr, wire := range map[string]string{
		"is_jit_enabled": "isJitEnabled", "script_only_access": "scriptOnlyAccess",
		"disable_output_capture": "disableOutputCapture", "disable_data_studio": "disableDataStudio",
		"disable_web_cli": "disableWebCli", "auto_approval": "autoApproval", "jit_multi_approver": "jitMultiApprover",
	} {
		for _, prior := range []bool{false, true} {
			t.Run(attr+map[bool]string{false: "/false-to-true", true: "/true-to-false"}[prior], func(t *testing.T) {
				api := &fakeAPI{}
				client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
					frame := completeEndpointPolicyFrame()
					frame[wire] = !prior
					_ = json.NewEncoder(w).Encode(frame)
				})
				d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{
					"name": "endpoint", "resource": "database", attr: prior,
				})
				d.SetId("ep-1")
				if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
					t.Fatal("endpoint refresh failed")
				}
				if d.Get(attr) != !prior {
					t.Error("endpoint-local security drift was suppressed")
				}
			})
		}
	}
}

func TestEndpointRefreshObservesZeroQuorumDrift(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		frame := completeEndpointPolicyFrame()
		frame["jitTotalApprovers"] = 3
		_ = json.NewEncoder(w).Encode(frame)
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{
		"name": "endpoint", "resource": "database", "jit_total_approvers": 0,
	})
	d.SetId("ep-1")
	if diags := ResourceAdaptiveSessionRead(context.Background(), d, client); diags.HasError() {
		t.Fatal("endpoint refresh failed")
	}
	if d.Get("jit_total_approvers") != 3 {
		t.Error("zero-to-nonzero endpoint quorum drift was suppressed")
	}
}

func TestEndpointDocumentedLiteralValuesMatchSchema(t *testing.T) {
	body, err := os.ReadFile("../../../templates/resources/endpoint.md.tmpl")
	if err != nil {
		t.Fatal("cannot read endpoint documentation template")
	}
	fences := regexp.MustCompile("(?s)```terraform\\s*\\n(.*?)```")
	seen := map[string]bool{}
	for _, match := range fences.FindAllStringSubmatch(string(body), -1) {
		file, diags := hclsyntax.ParseConfig([]byte(match[1]), "endpoint-example.tf", hcl.InitialPos)
		if diags.HasErrors() {
			t.Fatal("endpoint documentation contains invalid HCL syntax")
		}
		for _, block := range file.Body.(*hclsyntax.Body).Blocks {
			if block.Type != "resource" || len(block.Labels) != 2 || block.Labels[0] != "adaptive_endpoint" {
				continue
			}
			for _, key := range []string{"ttl", "cpu", "memory", "type"} {
				attr, exists := block.Body.Attributes[key]
				if !exists {
					continue
				}
				value, diags := attr.Expr.Value(nil)
				if diags.HasErrors() {
					continue
				} // Only literal examples are checked.
				seen[key] = true
				validator := ResourceAdaptiveSession().Schema[key].ValidateFunc
				if validator == nil {
					t.Errorf("%s lacks plan-time validation", key)
					continue
				}
				if _, errs := validator(value.AsString(), key); len(errs) > 0 {
					t.Errorf("documented endpoint %s literal is rejected by the schema", key)
				}
			}
		}
	}
	for _, key := range []string{"ttl", "cpu", "memory", "type"} {
		if !seen[key] {
			t.Errorf("no documented %s literal was checked", key)
		}
	}
}

func TestEndpointJITModeUsesPlannedValueNotRawOverride(t *testing.T) {
	d := maxJITDurationData(cty.ObjectVal(map[string]cty.Value{"jit_mode": cty.StringVal("session")}), "", false)
	if err := d.Set("jit_mode", "both"); err != nil {
		t.Fatal(err)
	}
	req, err := sessionRequestFromSchema(d)
	if err != nil {
		t.Fatal("could not build endpoint request")
	}
	if req.JITMode != "both" {
		t.Error("raw input overrode planned JIT mode retained by lifecycle.ignore_changes")
	}
}

func TestEndpointPolicyFieldsObserveWhenOmitted(t *testing.T) {
	for _, attr := range []string{"is_jit_enabled", "script_only_access", "disable_output_capture", "disable_data_studio", "disable_web_cli", "auto_approval", "jit_multi_approver", "jit_total_approvers", "max_jit_request_duration", "jit_approvers"} {
		s := ResourceAdaptiveSession().Schema[attr]
		if !s.Optional || !s.Computed || s.Default != nil || s.DefaultFunc != nil {
			t.Errorf("%s must observe/preserve imported policy unless configured explicitly", attr)
		}
	}
}
