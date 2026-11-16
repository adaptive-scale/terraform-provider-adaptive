package components

import (
	"fmt"
	"strings"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
)

// validateEndpointPolicyRead requires the values that the backend always
// replaces on update. Missing/null booleans must not become an invented false
// observation. Nullable approver slices are authoritative empty lists; missing
// lists are unknown. Current backend reads include every required field.
func validateEndpointPolicyRead(r *adaptive.SessionReadResponse) error {
	var missing []string
	for _, key := range []string{"resource", "sessionType", "isJitEnabled", "scriptOnlyAccess", "disableOutputCapture", "disableDataStudio", "disableWebCli", "accessApprovers"} {
		if !r.HasField(key) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cannot safely reconcile endpoint policy: read response omits authoritative fields %s; use a backend version that reports the complete endpoint-local policy", strings.Join(missing, ", "))
	}
	if r.Resource == "" || (r.SessionType != "cli" && r.SessionType != "client") {
		return fmt.Errorf("cannot safely reconcile endpoint policy: read response lacks a usable resource or canonical endpoint type")
	}
	return nil
}

// verifyEndpointPolicyWrite checks the authoritative object rather than treating
// HTTP success as proof that the backend persisted every requested policy.
func verifyEndpointPolicyWrite(req adaptive.CreateSessionRequest, r *adaptive.SessionReadResponse) error {
	if err := validateEndpointPolicyRead(r); err != nil {
		return err
	}
	var mismatched []string
	for _, f := range []struct {
		name              string
		requested, actual bool
	}{
		{"is_jit_enabled", req.IsJITEnabled, r.IsJITEnabled},
		{"script_only_access", req.ScriptOnlyAccess, r.ScriptOnlyAccess},
		{"disable_output_capture", req.DisableOutputCapture, r.DisableOutputCapture},
		{"disable_data_studio", req.DisableDataStudio, r.DisableDataStudio},
		{"disable_web_cli", req.DisableWebCLI, r.DisableWebCLI},
	} {
		if f.requested != f.actual {
			mismatched = append(mismatched, f.name)
		}
	}
	if req.SessionName != r.Name {
		mismatched = append(mismatched, "name")
	}
	if req.ResourceName != r.Resource {
		mismatched = append(mismatched, "resource")
	}
	if req.SessionType != r.SessionType {
		mismatched = append(mismatched, "type")
	}
	if !sameStringSet(req.AccessApprovers, r.AccessApprovers) {
		mismatched = append(mismatched, "jit_approvers")
	}
	if req.AutoApproval != nil && (!r.HasField("autoApproval") || *req.AutoApproval != r.AutoApproval) {
		mismatched = append(mismatched, "auto_approval")
	}
	if req.JITMultiApprover != nil && (!r.HasField("jitMultiApprover") || *req.JITMultiApprover != r.JITMultiApprover) {
		mismatched = append(mismatched, "jit_multi_approver")
	}
	if req.JITTotalApprovers != nil && (!r.HasField("jitTotalApprovers") || *req.JITTotalApprovers != r.JITTotalApprovers) {
		mismatched = append(mismatched, "jit_total_approvers")
	}
	// Duration and mode are intentionally omitempty in the supported read DTO.
	// A missing duration is an empty local override, not a workspace-effective cap.
	if req.MaxJITRequestDuration != nil && strings.TrimSpace(*req.MaxJITRequestDuration) != r.MaxJITRequestDuration {
		mismatched = append(mismatched, "max_jit_request_duration")
	}
	if req.JITMode != "" && req.JITMode != r.JITMode {
		mismatched = append(mismatched, "jit_mode")
	}
	if len(mismatched) > 0 {
		return fmt.Errorf("endpoint update returned success but read-back does not confirm requested fields %s; remote changes may be partial, refresh before retrying", strings.Join(mismatched, ", "))
	}
	return nil
}
