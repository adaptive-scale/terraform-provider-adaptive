package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const endpointEngineAPIPath = "/api/v1/terraform/session/"
const endpointEngineAddress = "adaptive_endpoint.test"

// These mappings deliberately describe the backend contract independently of
// the production request/response structs. Reads contain endpoint-local values,
// not effective workspace defaults; updates fully replace the five plain bools
// and approvers, but preserve omitted pointer policy and empty/omitted jit_mode.
var endpointEngineBools = map[string]string{
	"is_jit_enabled":         "isJitEnabled",
	"script_only_access":     "scriptOnlyAccess",
	"disable_output_capture": "disableOutputCapture",
	"disable_data_studio":    "disableDataStudio",
	"disable_web_cli":        "disableWebCli",
}
var endpointEnginePointers = map[string]string{
	"auto_approval":            "autoApproval",
	"jit_multi_approver":       "jitMultiApprover",
	"jit_total_approvers":      "jitTotalApprovers",
	"max_jit_request_duration": "maxJitRequestDuration",
}

// TestEndpointTerraformEngine runs real Terraform CLI import/plan/saved-plan
// apply against a local plugin and loopback-only fake API. No init, registry or
// real credentials. ADAPTIVE_ENGINE_PROVIDER_BINARY selects a baseline plugin.
// Run: ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^TestEndpointTerraformEngine$' -v -count=1
func TestEndpointTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("real Terraform CLI tests require ADAPTIVE_ENGINE_TEST=1")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("opted-in engine test requires terraform on PATH")
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline local provider build failed: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil || filepath.Base(bin) != "terraform-provider-adaptive" {
		t.Fatal("engine provider binary must be named terraform-provider-adaptive")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Fatal("engine provider binary is missing")
	}

	t.Run("import_unconfigured_policy_and_removal_preserve_full_replacement_fields", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineState(h, endpointEnginePolicy(true))
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "changed", "")
		endpointEnginePlan(h, api, 2, "tag.tfplan")
		h.run(0, "apply", "-auto-approve", "tag.tfplan")
		want := endpointEnginePolicy(true)
		want["userTags"] = []any{"changed"}
		api.assert(t, 1, want)
		endpointEngineClean(h, api)

		// Acquiring then removing explicit ownership must not clear the observed
		// full-replacement bools or approvers on the next unrelated update.
		endpointEngineConfig(h, "direct", "changed", endpointEngineBoolConfig(true)+"\njit_approvers = [\"approver-b@example.invalid\", \"approver-a@example.invalid\"]")
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "removed", "")
		remote := map[string]any{"isJitEnabled": false, "disableDataStudio": false, "accessApprovers": []any{"new@example.invalid"}}
		api.mutate(remote)
		endpointEnginePlan(h, api, 2, "removed.tfplan")
		h.run(0, "apply", "-auto-approve", "removed.tfplan")
		for key, value := range remote {
			want[key] = value
		}
		want["userTags"] = []any{"removed"}
		api.assert(t, 2, want)
		endpointEngineClean(h, api)
	})

	t.Run("explicit_false_drift_is_an_actual_five_field_update", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		zeros := map[string]any{}
		for _, key := range endpointEngineBools {
			zeros[key] = false
		}
		api.mutate(zeros)
		endpointEngineConfig(h, "direct", "initial", endpointEngineBoolConfig(false))
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineClean(h, api)
		trues := map[string]any{}
		for _, key := range endpointEngineBools {
			trues[key] = true
		}
		api.mutate(trues)
		endpointEnginePlan(h, api, 2, "drift.tfplan")
		expected := map[string][2]any{}
		for key := range endpointEngineBools {
			expected[key] = [2]any{true, false}
		}
		endpointEnginePlanValues(h, "drift.tfplan", expected)
		api.assert(t, 0, trues)
		h.run(0, "apply", "-auto-approve", "drift.tfplan")
		api.assert(t, 1, zeros)
		write := api.lastWrite(t)
		for key := range endpointEngineBools {
			if value, ok := write[key]; !ok || value != false {
				t.Fatalf("explicit false missing/wrong for wire key %s", key)
			}
		}
		endpointEngineClean(h, api)
	})

	t.Run("duration_clear_alone_schedules_update", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		api.mutate(map[string]any{"maxJitRequestDuration": "2h"})
		endpointEngineConfig(h, "direct", "initial", "max_jit_request_duration = \"\"")
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEnginePlan(h, api, 2, "clear-only.tfplan")
		h.run(0, "apply", "-auto-approve", "clear-only.tfplan")
		if value, present := api.lastWrite(t)["max_jit_request_duration"]; !present || value != "" {
			t.Fatal("duration-only clear did not reach the API as an explicit empty value")
		}
		api.assert(t, 1, map[string]any{"maxJitRequestDuration": "", "userTags": []any{"initial"}})
		endpointEngineState(h, map[string]any{"maxJitRequestDuration": ""})
		endpointEngineClean(h, api)
	})

	t.Run("duration_clear_respects_ignore_changes", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		api.mutate(map[string]any{"maxJitRequestDuration": "6h"})
		policy := "max_jit_request_duration = \"\"\nlifecycle { ignore_changes = [max_jit_request_duration] }"
		endpointEngineConfig(h, "direct", "initial", policy)
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "changed", policy)
		endpointEnginePlan(h, api, 2, "ignored-clear.tfplan")
		h.run(0, "apply", "-auto-approve", "ignored-clear.tfplan")
		endpointEngineOmitted(t, api.lastWrite(t))
		api.assert(t, 1, map[string]any{"maxJitRequestDuration": "6h", "userTags": []any{"changed"}})
		endpointEngineClean(h, api)
	})

	t.Run("pointer_policy_omission_explicit_zero_and_relinquishment", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineState(h, endpointEnginePolicy(true))
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "changed", "")
		endpointEnginePlan(h, api, 2, "pointer.tfplan")
		// Inject immediately before the POST is processed, after the last read.
		// A fake that round-trips observed pointer state would clobber this change.
		concurrent := map[string]any{"autoApproval": false, "jitMultiApprover": false, "jitTotalApprovers": float64(0), "maxJitRequestDuration": "6h", "jitMode": "script"}
		api.injectBeforeWrite(concurrent)
		h.run(0, "apply", "-auto-approve", "pointer.tfplan")
		endpointEngineOmitted(t, api.lastWrite(t))
		api.assert(t, 1, concurrent)
		api.assertInjected(t, 1)
		endpointEngineClean(h, api)

		// False multi-approver plus zero quorum is a safe explicit clear; an empty
		// duration clears the endpoint override rather than omitting the JSON key.
		api.mutate(map[string]any{"autoApproval": true, "jitMultiApprover": true, "jitTotalApprovers": float64(3), "maxJitRequestDuration": "4h"})
		explicit := "auto_approval = false\njit_multi_approver = false\njit_total_approvers = 0\nmax_jit_request_duration = \"\"\njit_mode = \"session\""
		endpointEngineConfig(h, "direct", "explicit", explicit)
		endpointEnginePlan(h, api, 2, "zero.tfplan")
		endpointEnginePlanValues(h, "zero.tfplan", map[string][2]any{
			"auto_approval": {true, false}, "jit_multi_approver": {true, false},
			"jit_total_approvers": {float64(3), float64(0)}, "jit_mode": {"script", "session"},
		})
		h.run(0, "apply", "-auto-approve", "zero.tfplan")
		cleared := map[string]any{"autoApproval": false, "jitMultiApprover": false, "jitTotalApprovers": float64(0), "maxJitRequestDuration": "", "jitMode": "session"}
		write := api.lastWrite(t)
		for wire, read := range endpointEnginePointers {
			if value, ok := write[wire]; !ok || !reflect.DeepEqual(value, cleared[read]) {
				t.Errorf("explicit pointer clear missing/wrong: %s present=%t value=%v want=%v", wire, ok, value, cleared[read])
			}
		}
		api.assert(t, 2, cleared)
		if write["jit_mode"] != "session" {
			t.Fatal("configured nonempty jit_mode was not written")
		}
		endpointEngineClean(h, api)

		endpointEngineConfig(h, "direct", "removed", "")
		endpointEnginePlan(h, api, 2, "relinquish.tfplan")
		renewed := map[string]any{"autoApproval": true, "jitMultiApprover": true, "jitTotalApprovers": float64(2), "maxJitRequestDuration": "8h", "jitMode": "both"}
		api.injectBeforeWrite(renewed)
		h.run(0, "apply", "-auto-approve", "relinquish.tfplan")
		endpointEngineOmitted(t, api.lastWrite(t))
		api.assert(t, 3, renewed)
		api.assertInjected(t, 2)
		endpointEngineClean(h, api)
	})

	t.Run("jit_mode_observes_empty_old_state_and_empty_null_stop_managing", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		api.mutate(map[string]any{"jitMode": ""})
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineState(h, map[string]any{"jitMode": ""})
		api.mutate(map[string]any{"jitMode": "script"})
		endpointEngineClean(h, api)
		h.run(0, "apply", "-refresh-only", "-auto-approve")
		endpointEngineState(h, map[string]any{"jitMode": "script"})
		for _, inherited := range []string{"jit_mode = \"\"", "jit_mode = null"} {
			endpointEngineConfig(h, "direct", "initial", inherited)
			endpointEngineClean(h, api)
		}
		endpointEngineConfig(h, "direct", "initial", "jit_mode = \"session\"")
		endpointEnginePlan(h, api, 2, "mode.tfplan")
		endpointEnginePlanValues(h, "mode.tfplan", map[string][2]any{"jit_mode": {"script", "session"}})
		h.run(0, "apply", "-auto-approve", "mode.tfplan")
		api.assert(t, 1, map[string]any{"jitMode": "session"})
		endpointEngineClean(h, api)
		for index, inherited := range []string{"jit_mode = \"\"", "jit_mode = null"} {
			tag := fmt.Sprintf("inherit-%d", index)
			endpointEngineConfig(h, "direct", tag, inherited)
			endpointEnginePlan(h, api, 2, "inherit.tfplan")
			api.injectBeforeWrite(map[string]any{"jitMode": "both"})
			h.run(0, "apply", "-auto-approve", "inherit.tfplan")
			endpointEngineOmitted(t, api.lastWrite(t))
			api.assert(t, index+2, map[string]any{"jitMode": "both", "userTags": []any{tag}})
			endpointEngineClean(h, api)
		}
	})

	t.Run("direct_cli_aliases_are_noops", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		for _, typ := range []string{"direct", "cli", "direct"} {
			endpointEngineConfig(h, typ, "initial", "")
			endpointEngineClean(h, api)
		}
		api.assert(t, 0, map[string]any{"sessionType": "cli"})
	})
	for _, typ := range []string{"client", "services", "not-a-type"} {
		t.Run("reject_type_change_during_plan_"+typ, func(t *testing.T) {
			h, api := newEndpointEngine(t, terraform, bin, true)
			endpointEngineConfig(h, "direct", "initial", "")
			h.run(0, "import", endpointEngineAddress, engineID)
			endpointEngineConfig(h, typ, "initial", "")
			out := endpointEnginePlan(h, api, 1, "")
			if !strings.Contains(strings.ToLower(out), "type") {
				t.Fatalf("wrong rejection diagnostic:\n%s", engineSafe(out))
			}
			api.assert(t, 0, map[string]any{"sessionType": "cli"})
		})
	}
	for _, invalidType := range []string{"not-a-type", "services"} {
		t.Run("invalid_type_rejected_before_create_"+invalidType, func(t *testing.T) {
			h, api := newEndpointEngine(t, terraform, bin, false)
			endpointEngineConfig(h, invalidType, "initial", "")
			out := endpointEnginePlan(h, api, 1, "")
			if !strings.Contains(strings.ToLower(out), "type") {
				t.Fatalf("wrong invalid type diagnostic:\n%s", engineSafe(out))
			}
			api.assert(t, 0, nil)
		})
	}

	t.Run("incomplete_policy_read_preserves_state_and_blocks_update", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		api.omitReadField("disableDataStudio")
		out := h.run(1, "apply", "-refresh-only", "-auto-approve")
		if !strings.Contains(strings.ToLower(out), "disable") {
			t.Fatalf("incomplete policy read failed without the missing field diagnostic:\n%s", engineSafe(out))
		}
		endpointEngineState(h, endpointEnginePolicy(true))
		endpointEngineState(h, map[string]any{"id": engineID})
		api.assert(t, 0, endpointEnginePolicy(true))

		// The saved plan was built with a complete DTO, so this failure can only
		// come from the apply-time preflight read, not the planning refresh.
		api.omitReadField("")
		endpointEngineConfig(h, "direct", "changed", "")
		endpointEnginePlan(h, api, 2, "incomplete.tfplan")
		api.omitReadField("disableDataStudio")
		h.run(1, "apply", "-auto-approve", "incomplete.tfplan")
		endpointEngineState(h, endpointEnginePolicy(true))
		endpointEngineState(h, map[string]any{"id": engineID})
		api.assert(t, 0, map[string]any{"userTags": []any{"initial"}})
	})

	t.Run("http_200_ignored_policy_write_must_fail_apply", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, true)
		endpointEngineConfig(h, "direct", "initial", "")
		h.run(0, "import", endpointEngineAddress, engineID)
		endpointEngineConfig(h, "direct", "changed", endpointEngineBoolConfig(false))
		endpointEnginePlan(h, api, 2, "ignored.tfplan")
		api.ignorePolicyWrites()
		out := h.run(1, "apply", "-auto-approve", "ignored.tfplan")
		if !strings.Contains(strings.ToLower(out), "policy") && !strings.Contains(strings.ToLower(out), "jit") {
			t.Fatalf("ignored policy write failed without policy verification diagnostic:\n%s", engineSafe(out))
		}
		// The API accepted the POST and tag, but swallowed its policy writes.
		// Only an authoritative read after POST can distinguish this from success.
		want := endpointEnginePolicy(true)
		want["userTags"] = []any{"changed"}
		api.assert(t, 1, want)
		write := api.lastWrite(t)
		for key := range endpointEngineBools {
			if value, ok := write[key]; !ok || value != false {
				t.Fatalf("negative apply never actually requested false for %s", key)
			}
		}
	})

	t.Run("create_update_lists_preserve_order_as_sets_and_remove_members", func(t *testing.T) {
		h, api := newEndpointEngine(t, terraform, bin, false)
		api.reverseLists = true
		args := "users = [\"b@example.invalid\", \"a@example.invalid\"]\ngroups = [\"group-b\", \"group-a\"]\njit_approvers = [\"approver-b@example.invalid\", \"approver-a@example.invalid\"]"
		endpointEngineConfig(h, "direct", "initial", args)
		h.run(0, "apply", "-auto-approve")
		api.assert(t, 1, map[string]any{"sessionUsers": []any{"b@example.invalid", "a@example.invalid"}, "groups": []any{"group-b", "group-a"}, "accessApprovers": []any{"approver-b@example.invalid", "approver-a@example.invalid"}, "sessionType": "cli"})
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "removed", "users = [\"a@example.invalid\"]\ngroups = [\"group-a\"]\njit_approvers = [\"approver-a@example.invalid\"]")
		endpointEnginePlan(h, api, 2, "lists.tfplan")
		h.run(0, "apply", "-auto-approve", "lists.tfplan")
		api.assert(t, 2, map[string]any{"sessionUsers": []any{"a@example.invalid"}, "groups": []any{"group-a"}, "accessApprovers": []any{"approver-a@example.invalid"}})
		endpointEngineClean(h, api)
		endpointEngineConfig(h, "direct", "empty", "users = []\ngroups = []\njit_approvers = []")
		endpointEnginePlan(h, api, 2, "empty.tfplan")
		h.run(0, "apply", "-auto-approve", "empty.tfplan")
		api.assert(t, 3, map[string]any{"sessionUsers": []any{}, "groups": []any{}, "accessApprovers": []any{}})
		if got := api.lastWrite(t)["access_approvers"]; !reflect.DeepEqual(got, []any{}) {
			t.Fatal("explicit empty approvers missing from full replacement request")
		}
		endpointEngineClean(h, api)
	})
}

func endpointEngineBoolConfig(value bool) string {
	return fmt.Sprintf("is_jit_enabled = %t\nscript_only_access = %t\ndisable_output_capture = %t\ndisable_data_studio = %t\ndisable_web_cli = %t", value, value, value, value, value)
}
func endpointEngineConfig(h *secretEngine, typ, tag, args string) {
	h.t.Helper()
	engineWrite(h.t, filepath.Join(h.dir, "main.tf"), fmt.Sprintf(`terraform {
 required_providers {
  adaptive = { source = "adaptive-scale/adaptive" }
 }
}
provider "adaptive" {
 service_token = %q
 workspace_url = %q
}
resource "adaptive_endpoint" "test" {
 name = "engine-endpoint"
 resource = "engine-resource"
 type = %q
 tags = [%q]
 %s
}
`, engineToken, h.url, typ, tag, args))
}

// Check plan purity, including error plans: planning may read, never POST or
// alter remote state. Saved plans are inspected using Terraform's own JSON.
func endpointEnginePlan(h *secretEngine, api *endpointEngineAPI, want int, file string) string {
	h.t.Helper()
	before, writes, reads := api.snapshot()
	args := []string{"plan", "-detailed-exitcode"}
	if file != "" {
		args = append(args, "-out="+file)
	}
	out := h.run(want, args...)
	after, afterWrites, afterReads := api.snapshot()
	if writes != afterWrites || !reflect.DeepEqual(before, after) {
		h.t.Fatal("Terraform plan mutated the fake API")
	}
	if want != 1 && afterReads <= reads {
		h.t.Fatal("Terraform plan did not refresh the endpoint")
	}
	return out
}
func endpointEngineClean(h *secretEngine, api *endpointEngineAPI) {
	h.t.Helper()
	endpointEnginePlan(h, api, 0, "")
}
func endpointEnginePlanValues(h *secretEngine, file string, values map[string][2]any) {
	h.t.Helper()
	var plan struct {
		Changes []struct {
			Address string `json:"address"`
			Change  struct {
				Actions      []string       `json:"actions"`
				Before       map[string]any `json:"before"`
				After        map[string]any `json:"after"`
				AfterUnknown map[string]any `json:"after_unknown"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal([]byte(h.run(0, "show", "-json", file)), &plan); err != nil {
		h.t.Fatal("cannot decode actual Terraform plan JSON")
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Address != endpointEngineAddress || !reflect.DeepEqual(plan.Changes[0].Change.Actions, []string{"update"}) {
		h.t.Fatal("actual Terraform JSON plan did not propose a single in-place endpoint update")
	}
	for key, want := range values {
		change := plan.Changes[0].Change
		if unknown := change.AfterUnknown[key]; unknown != nil && unknown != false {
			h.t.Fatalf("actual Terraform JSON plan %s is unknown, not a configured value", key)
		}
		if !reflect.DeepEqual(change.Before[key], want[0]) || !reflect.DeepEqual(change.After[key], want[1]) {
			h.t.Errorf("actual Terraform JSON plan %s: before=%v after=%v; want before=%v after=%v", key, change.Before[key], change.After[key], want[0], want[1])
		}
	}
}
func endpointEngineState(h *secretEngine, expected map[string]any) {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "terraform.tfstate"))
	if err != nil {
		h.t.Fatal(err)
	}
	var state struct {
		Resources []struct {
			Type      string `json:"type"`
			Instances []struct {
				Attributes map[string]any `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(data, &state) != nil || len(state.Resources) != 1 || state.Resources[0].Type != "adaptive_endpoint" || len(state.Resources[0].Instances) != 1 {
		h.t.Fatal("unexpected persisted endpoint state shape")
	}
	keys := map[string]string{"id": "id", "accessApprovers": "jit_approvers", "autoApproval": "auto_approval", "jitMultiApprover": "jit_multi_approver", "jitTotalApprovers": "jit_total_approvers", "maxJitRequestDuration": "max_jit_request_duration", "jitMode": "jit_mode"}
	for wire, read := range endpointEngineBools {
		keys[read] = wire
	}
	for read, want := range expected {
		key, ok := keys[read]
		if !ok {
			h.t.Fatalf("no state mapping for %s", read)
		}
		got := state.Resources[0].Instances[0].Attributes[key]
		if !reflect.DeepEqual(got, want) {
			h.t.Fatalf("persisted endpoint %s=%v, want %v", key, got, want)
		}
	}
}
func endpointEngineOmitted(t *testing.T, write map[string]any) {
	t.Helper()
	for key := range endpointEnginePointers {
		if _, ok := write[key]; ok {
			t.Fatalf("unconfigured pointer policy was serialized: %s", key)
		}
	}
	if _, ok := write["jit_mode"]; ok {
		t.Fatal("unmanaged/empty/null jit_mode must not be serialized")
	}
}
func endpointEnginePolicy(nondefault bool) map[string]any {
	out := map[string]any{"autoApproval": nondefault, "jitMultiApprover": nondefault, "jitTotalApprovers": float64(0), "maxJitRequestDuration": "", "jitMode": "", "accessApprovers": []any{}}
	for _, key := range endpointEngineBools {
		out[key] = nondefault
	}
	if nondefault {
		out["jitTotalApprovers"], out["maxJitRequestDuration"], out["jitMode"] = float64(2), "2h", "both"
		out["accessApprovers"] = []any{"approver-b@example.invalid", "approver-a@example.invalid"}
	}
	return out
}

type endpointEngineAPI struct {
	omitRead             string
	ignorePolicy         bool
	mu                   sync.Mutex
	exists, reverseLists bool
	remote               map[string]any
	beforeWrite          map[string]any
	reads, injected      int
	calls, errors        []string
	writes               []map[string]any
}

func newEndpointEngine(t *testing.T, terraform, bin string, exists bool) (*secretEngine, *endpointEngineAPI) {
	t.Helper()
	remote := endpointEnginePolicy(exists)
	for key, value := range map[string]any{
		"id": engineID, "name": "engine-endpoint", "resource": "engine-resource", "status": "created", "sessionType": "cli",
		"cluster": "", "authorization": "", "ttl": "", "idleTimeout": "", "pauseTimeout": "", "memory": "", "cpu": "", "storage": "",
		"sessionUsers": []any{}, "groups": []any{}, "userTags": []any{"initial"}, "public": false, "exposed": false, "exposeType": "", "exposeStatus": "",
	} {
		remote[key] = value
	}
	api := &endpointEngineAPI{exists: exists, remote: remote}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	// Reuse the existing isolated CLI environment and dev override verbatim.
	h := newSecretEngine(t, terraform, bin, false)
	h.url = server.URL
	t.Cleanup(func() {
		api.mu.Lock()
		defer api.mu.Unlock()
		if len(api.errors) != 0 {
			t.Errorf("endpoint fake API contract violations: %v", api.errors)
		}
		t.Logf("endpoint API calls: %s; reads=%d mutations=%d injected=%d", strings.Join(api.calls, ", "), api.reads, len(api.writes), api.injected)
	})
	return h, api
}
func endpointEngineCopy(src map[string]any) map[string]any {
	data, err := json.Marshal(src)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}
func (a *endpointEngineAPI) omitReadField(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.omitRead = key
}
func (a *endpointEngineAPI) ignorePolicyWrites() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ignorePolicy = true
}
func (a *endpointEngineAPI) snapshot() (map[string]any, int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return endpointEngineCopy(a.remote), len(a.writes), a.reads
}
func (a *endpointEngineAPI) mutate(values map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, value := range endpointEngineCopy(values) {
		a.remote[key] = value
	}
}
func (a *endpointEngineAPI) injectBeforeWrite(values map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.beforeWrite = endpointEngineCopy(values)
}
func (a *endpointEngineAPI) assertInjected(t *testing.T, want int) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.injected != want || a.beforeWrite != nil {
		t.Fatalf("concurrent server mutation not exercised: injected=%d want=%d", a.injected, want)
	}
}
func (a *endpointEngineAPI) lastWrite(t *testing.T) map[string]any {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.writes) == 0 {
		t.Fatal("no actual endpoint POST captured")
	}
	return endpointEngineCopy(a.writes[len(a.writes)-1])
}
func (a *endpointEngineAPI) assert(t *testing.T, mutations int, expected map[string]any) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.errors) != 0 || len(a.writes) != mutations {
		t.Fatalf("endpoint fake API writes=%d want=%d violations=%v", len(a.writes), mutations, a.errors)
	}
	for key, want := range expected {
		if !reflect.DeepEqual(a.remote[key], want) {
			t.Fatalf("endpoint backend %s=%v, want %v", key, a.remote[key], want)
		}
	}
}
func (a *endpointEngineAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	fail := func(message string) {
		a.errors = append(a.errors, message)
		http.Error(w, message, http.StatusBadRequest)
	}
	if r.Header.Get("Authorization") != engineToken {
		fail("missing explicit fake authorization")
		return
	}
	if r.URL.RawQuery != "" {
		fail("unexpected query")
		return
	}
	if r.Method == "GET" && r.URL.Path == endpointEngineAPIPath+"read/"+engineID {
		a.reads++
		if !a.exists {
			http.NotFound(w, r)
			return
		}
		dto := endpointEngineCopy(a.remote)
		delete(dto, a.omitRead)
		if a.reverseLists {
			for _, key := range []string{"sessionUsers", "groups", "accessApprovers"} {
				list := dto[key].([]any)
				for left, right := 0, len(list)-1; left < right; left, right = left+1, right-1 {
					list[left], list[right] = list[right], list[left]
				}
			}
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(dto)
		return
	}
	create, update := r.URL.Path == endpointEngineAPIPath+"create", r.URL.Path == endpointEngineAPIPath+"update/"+engineID
	if r.Method != "POST" || (!create && !update) {
		fail("unexpected endpoint/method")
		return
	}
	if create == a.exists {
		fail("incorrect create/update lifecycle")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		fail("wrong mutation content type")
		return
	}
	var request map[string]any
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&request) != nil || request == nil {
		fail("invalid mutation JSON")
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		fail("trailing mutation JSON")
		return
	}
	types := map[string]string{
		"sessionName": "string", "resourceName": "string", "clusterName": "string", "authorizationName": "string", "sessionTTL": "string", "sessionType": "string",
		"sessionUsers": "list", "access_approvers": "list", "memory": "string", "cpu": "string", "usertags": "list", "pause_timeout": "string", "groups": "list", "idle_timeout": "string",
		"jit_mode": "string", "auto_approval": "bool", "jit_multi_approver": "bool", "jit_total_approvers": "number", "max_jit_request_duration": "string",
	}
	for key := range endpointEngineBools {
		types[key] = "bool"
	}
	for key, value := range request {
		valid := false
		switch types[key] {
		case "string":
			_, valid = value.(string)
		case "bool":
			_, valid = value.(bool)
		case "number":
			number, ok := value.(float64)
			valid = ok && number >= 0 && number == float64(int(number))
		case "list":
			if list, ok := value.([]any); ok {
				valid = true
				for _, element := range list {
					str, ok := element.(string)
					if !ok || str == "" {
						valid = false
					}
				}
			}
		}
		if !valid {
			fail("unexpected mutation key/type: " + key)
			return
		}
	}
	for key := range endpointEngineBools {
		if _, ok := request[key]; !ok {
			fail("missing full replacement bool: " + key)
			return
		}
	}
	for _, key := range []string{"access_approvers", "memory", "cpu", "usertags"} {
		if _, ok := request[key]; !ok {
			fail("missing full replacement field: " + key)
			return
		}
	}
	if request["sessionName"] != "engine-endpoint" || request["resourceName"] != "engine-resource" {
		fail("wrong backend identity names")
		return
	}
	if request["sessionType"] != "cli" && request["sessionType"] != "client" {
		fail("invalid backend session type")
		return
	}
	if mode, ok := request["jit_mode"]; ok && mode != "" && mode != "session" && mode != "script" && mode != "both" {
		fail("invalid backend JIT mode")
		return
	}
	if a.beforeWrite != nil {
		for key, value := range a.beforeWrite {
			a.remote[key] = value
		}
		a.beforeWrite = nil
		a.injected++
	}
	// These fields are full replacement: absent list fields become empty lists.
	for wire, read := range endpointEngineBools {
		if !a.ignorePolicy {
			a.remote[read] = request[wire]
		}
	}
	for wire, read := range map[string]string{"access_approvers": "accessApprovers", "sessionUsers": "sessionUsers", "groups": "groups", "usertags": "userTags"} {
		if a.ignorePolicy && wire == "access_approvers" {
			continue
		}
		value, ok := request[wire]
		if !ok {
			value = []any{}
		}
		a.remote[read] = value
	}
	for wire, read := range endpointEnginePointers {
		if value, present := request[wire]; present && !a.ignorePolicy {
			a.remote[read] = value
		}
	}
	if value, ok := request["jit_mode"].(string); ok && value != "" && !a.ignorePolicy {
		a.remote["jitMode"] = value
	}
	for wire, read := range map[string]string{"sessionName": "name", "resourceName": "resource", "sessionType": "sessionType", "memory": "memory", "cpu": "cpu", "clusterName": "cluster", "authorizationName": "authorization", "sessionTTL": "ttl", "pause_timeout": "pauseTimeout", "idle_timeout": "idleTimeout"} {
		if wire == "sessionType" && !create {
			continue
		} // The backend cannot change type in place.
		value, present := request[wire]
		if !present {
			value = ""
		}
		a.remote[read] = value
	}
	a.exists = true
	a.writes = append(a.writes, endpointEngineCopy(request))
	_ = json.NewEncoder(w).Encode(map[string]any{"id": engineID})
}
