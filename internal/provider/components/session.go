package components

import (
	"context"
	"fmt"
	"time"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
)

const (
	SessionTTLOptionNone = ""
	// hours
	SessionTTLOption3Hours = "3h"
	SessionTTLOption6Hours = "6h"
	// days
	SessionTTLOption1days   = "1d"
	SessionTTLOption3days   = "3d"
	SessionTTLOption7days   = "7d"
	SessionTTLOption30days  = "30d"
	SessionTTLOption60days  = "60d"
	SessionTTLOption90days  = "90d"
	SessionTTLOption180days = "180d"
	SessionTTLOption365days = "365d"
)

var validTTLOptions = []string{
	SessionTTLOptionNone,
	SessionTTLOption3Hours,
	SessionTTLOption6Hours,
	SessionTTLOption1days,
	SessionTTLOption3days,
	SessionTTLOption7days,
	SessionTTLOption30days,
	SessionTTLOption60days,
	SessionTTLOption90days,
	SessionTTLOption180days,
	SessionTTLOption365days,
}

const (
	SessionTypeDefault  = "direct"
	SessionTypeDirect   = "direct"
	SessionTypeScript   = "script"
	SessionTypeClient   = "client"
	SessionTypeCLI      = "cli"
	SessionTypeServices = "services"
)

const (
	EndpointMemoryDefault = "256Mi"
	EndpointMemory128     = "128Mi"
	EndpointMemory256     = "256Mi"
	EndpointMemory512     = "512Mi"
	EndpointMemory1024    = "1024Mi"
	EndpointMemory2048    = "2048Mi"
	EndpointMemory4096    = "4096Mi"
	EndpointMemory8192    = "8192Mi"
)

var validMemoryValues = []string{
	EndpointMemory128,
	EndpointMemory256,
	EndpointMemory512,
	EndpointMemory1024,
	EndpointMemory2048,
	EndpointMemory4096,
	EndpointMemory8192,
}

var validIdleTimeoutValues = []string{
	"15m",
	"30m",
	"1h",
	"2h",
	"3h",
	"6h",
	"1d",
	"3d",
	"7d",
	"15d",
	"30d",
	"60d",
	"90d",
	"180d",
	"365d",
	"99999d",
}

const (
	EndpointCPUDefault = "0.5"
	EndpointCPU0125    = "0.125"
	EndpointCPU025     = "0.25"
	EndpointCPU050     = "0.5"
	EndpointCPU100     = "1"
	EndpointCPU200     = "2"
	EndpointCPU400     = "4"
	EndpointCPU800     = "8"
)

var validCPUValues = []string{
	EndpointCPU0125,
	EndpointCPU025,
	EndpointCPU050,
	EndpointCPU100,
	EndpointCPU200,
	EndpointCPU400,
	EndpointCPU800,
}

var validPauseTimeoutValues = []string{
	"15m",
	"30m",
	"1h",
	"2h",
	"3h",
	"6h",
	"1d",
	"3d",
	"7d",
	"15d",
	"30d",
	"60d",
	"90d",
	"180d",
	"365d",
	"99999d",
}

func ResourceAdaptiveSession() *schema.Resource {
	return &schema.Resource{
		CreateContext: ResourceAdaptiveSessionCreate,
		ReadContext:   ResourceAdaptiveSessionRead,
		UpdateContext: ResourceAdaptiveSessionUpdate,
		DeleteContext: ResourceAdaptiveSessionDelete,
		CustomizeDiff: endpointContractCustomizeDiff,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the session to create.",
			},
			"resource": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The resource used to create the session.",
			},
			"type": {
				Type:     schema.TypeString,
				Default:  SessionTypeDefault,
				Optional: true,
				ValidateFunc: func(i interface{}, k string) ([]string, []error) {
					typ, ok := i.(string)
					if !ok || !isValidSessionType(typ) {
						return nil, []error{fmt.Errorf("%s must be one of direct, cli, client", k)}
					}
					return nil, nil
				},
				DiffSuppressFunc: func(_ string, old, new string, _ *schema.ResourceData) bool {
					a, aOK := getSessionType(old)
					b, bOK := getSessionType(new)
					return aOK && bOK && a == b
				},
				Description: "The type of session to create. One of: direct (the default), cli, client. " +
					"`direct`, `cli`, and an empty string are the same session type.",
			},
			"ttl": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					if _, ok := i.(string); !ok {
						return nil, []error{fmt.Errorf("ttl must be a string")}
					}

					// make sure it's valid value
					if !slices.Contains(
						validTTLOptions, i.(string)) {
						return nil, []error{fmt.Errorf("ttl must be one of %v", validTTLOptions)}
					}

					return nil, nil
				},
				Description: "The time-to-live (TTL) for the session. The session will be automatically terminated after this time period. If not set, defaults to 90 days.",
			},
			"authorization": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The authorization to use when creating the session.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Default:     "",
				Optional:    true,
				Description: "The cluster in which this session should be created. If not provided will be set to default cluster set in workspace settings of the user's workspace",
			},
			"idle_timeout": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The time after which the session will be automatically terminated if no user is connected. Defaults to never timeout.",
				ValidateFunc: func(i interface{}, k string) (ws []string, es []error) {
					if _, ok := i.(string); !ok {
						es = append(es, fmt.Errorf("idle_timeout must be a string"))
					}

					// make sure it's valid value
					if !slices.Contains(
						validIdleTimeoutValues, i.(string)) {
						es = append(es, fmt.Errorf("idle_timeout must be one of %v", validIdleTimeoutValues))
					}
					return
				},
			},
			"users": {
				Type:     schema.TypeList,
				Computed: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The list of users associated with the adaptive endpoint",
			},
			"groups": {
				Type:     schema.TypeList,
				Computed: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The list of groups associated with the adaptive endpoint",
			},
			"is_jit_enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeBool},
				Description: "Whether Just-In-Time access is enabled for the session",
			},
			"jit_approvers": {
				Type:     schema.TypeList,
				Computed: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The list of user emails who can approve Just-In-Time access requests",
			},
			"pause_timeout": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The time after which the session will be paused if no user has connected to it. Defaults to never pause.",
				ValidateFunc: func(i interface{}, k string) (ws []string, es []error) {
					if _, ok := i.(string); !ok {
						es = append(es, fmt.Errorf("pause_timeout must be a string"))
					}

					// make sure it's valid value
					if !slices.Contains(
						validPauseTimeoutValues, i.(string)) {
						es = append(es, fmt.Errorf("pause_timeout must be one of %v", validPauseTimeoutValues))
					}

					return
				},
			},
			"memory": {
				Type:    schema.TypeString,
				Default: "",
				ValidateFunc: func(i interface{}, k string) (ws []string, es []error) {
					if _, ok := i.(string); !ok {
						es = append(es, fmt.Errorf("memory must be a string"))
					}

					// make sure it's valid value
					if !slices.Contains(
						validMemoryValues, i.(string)) {
						es = append(es, fmt.Errorf("memory must be one of %v", validMemoryValues))
					}

					return
				},
				Optional:    true,
				Description: "Memory of endpoint pod",
			},
			"cpu": {
				Type:    schema.TypeString,
				Default: "",
				ValidateFunc: func(i interface{}, k string) (ws []string, es []error) {
					if _, ok := i.(string); !ok {
						es = append(es, fmt.Errorf("cpu must be a string"))
					}

					// make sure it's valid value
					if !slices.Contains(
						validCPUValues, i.(string)) {
						es = append(es, fmt.Errorf("cpu must be one of %v", validCPUValues))
					}

					return
				},
				Optional:    true,
				Description: "CPU of endpoint pod",
			},
			"script_only_access": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Description: "Whether the endpoint should only be accessible via script",
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Optional tags",
			},
			"disable_output_capture": {
				Type:     schema.TypeBool,
				Computed: true,
				Optional: true,
				Description: "Do not retain terminal output for this endpoint. The effective value is OR'd with " +
					"the workspace-level setting: an endpoint cannot re-enable capture that the workspace disabled.",
			},
			"disable_data_studio": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Description: "Hide Data Studio for this endpoint. Only meaningful for `cli` endpoints.",
			},
			"disable_web_cli": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Description: "Hide \"Connect via Web CLI\" for this endpoint. Only meaningful for `cli` endpoints.",
			},
			"jit_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Which access paths require Just-In-Time approval. One of: session, script, both. Omitted or empty means unmanaged and observes the server value; the API cannot clear this setting with an empty string.",
				DiffSuppressFunc: func(_ string, _, new string, _ *schema.ResourceData) bool {
					return new == ""
				},
				ValidateFunc: func(i interface{}, k string) (ws []string, es []error) {
					v, ok := i.(string)
					if !ok {
						return nil, []error{fmt.Errorf("%s must be a string", k)}
					}
					if !slices.Contains(validJITModes, v) {
						return nil, []error{fmt.Errorf("%s must be one of %v", k, validJITModes[1:])}
					}
					return nil, nil
				},
			},
			"max_jit_request_duration": {
				Type:        schema.TypeString,
				Computed:    true,
				Optional:    true,
				Description: "Maximum duration of a Just-In-Time access request for this endpoint. Omit to preserve the existing setting; an empty string clears the endpoint override and inherits the workspace setting.",
			},
			"auto_approval": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Description: "Automatically approve Just-In-Time access requests.",
			},
			"jit_multi_approver": {
				Type:        schema.TypeBool,
				Computed:    true,
				Optional:    true,
				Description: "Require more than one approver for Just-In-Time access requests.",
			},
			"jit_total_approvers": {
				Type:        schema.TypeInt,
				Computed:    true,
				Optional:    true,
				Description: "Number of approvals required when `jit_multi_approver` is enabled.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Current lifecycle status of the endpoint.",
			},
			"exposed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the endpoint is exposed as a service.",
			},
			"expose_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "How the endpoint is exposed (e.g. LoadBalancer).",
			},
			"expose_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Provisioning status of the service exposure.",
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

var validJITModes = []string{"", "session", "script", "both"}

// The update handler does not apply SessionType. Reject transitions rather
// than reporting success or automatically destroying a running endpoint.
func validateEndpointTypeChange(old, next string) error {
	newWire, valid := getSessionType(next)
	if !valid {
		return fmt.Errorf("invalid endpoint type: %s; use direct, cli, or client", next)
	}
	oldWire, oldValid := getSessionType(old)
	if oldValid && oldWire != newWire {
		return fmt.Errorf("endpoint type cannot be changed in place; create a separate endpoint with the desired type and migrate explicitly")
	}
	return nil
}

func endpointContractCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	// SDKv2 otherwise treats an empty Optional+Computed string as unset,
	// losing the API's explicit-clear operation. Preserve raw configuration
	// intent without inferring ownership from an observed state value.
	raw := d.GetRawConfig()
	if !raw.IsNull() && raw.IsKnown() && raw.Type().IsObjectType() && raw.Type().HasAttribute("max_jit_request_duration") {
		v := raw.GetAttr("max_jit_request_duration")
		if v.IsKnown() && !v.IsNull() && v.AsString() == "" {
			// Core has already applied lifecycle.ignore_changes to the proposed
			// plan. Do not replace a retained known override with clear intent.
			plan := d.GetRawPlan()
			if !plan.IsNull() && plan.IsKnown() && plan.Type().IsObjectType() && plan.Type().HasAttribute("max_jit_request_duration") {
				p := plan.GetAttr("max_jit_request_duration")
				if p.IsKnown() && !p.IsNull() && p.AsString() != "" {
					return endpointTypeDiff(d)
				}
			}
			if err := d.SetNew("max_jit_request_duration", ""); err != nil {
				return err
			}
		}
	}
	return endpointTypeDiff(d)
}

func endpointTypeDiff(d *schema.ResourceDiff) error {
	if !d.NewValueKnown("type") {
		return nil
	}
	old, next := d.GetChange("type")
	if d.Id() == "" {
		if !isValidSessionType(next.(string)) {
			return fmt.Errorf("invalid endpoint type: %s; use direct, cli, or client", next)
		}
		return nil
	}
	return validateEndpointTypeChange(old.(string), next.(string))
}

func isValidSessionType(t string) bool {
	return t == "" || t == SessionTypeDirect || t == SessionTypeClient || t == SessionTypeCLI
}

func getSessionType(t string) (string, bool) {
	if t == "" {
		return "cli", true
	}

	if isValidSessionType(t) {
		switch t {
		case SessionTypeDirect:
			return "cli", true
		case SessionTypeClient:
			return "client", true
		case SessionTypeCLI:
			return "cli", true
		default:
			return "", false
		}
	}

	return "", false

}

// sessionRequestFromSchema builds the API request from resource state. The
// pointer-valued fields are only populated when the configuration actually sets
// them, so an unset toggle keeps whatever the workspace defaults to instead of
// being written back as false/0.
func sessionRequestFromSchema(d *schema.ResourceData) (adaptive.CreateSessionRequest, error) {
	sName, err := integrations.NameFromSchema(d)
	if err != nil {
		return adaptive.CreateSessionRequest{}, err
	}

	userEmails, err := nonEmptyStringList(d, "users", "email")
	if err != nil {
		return adaptive.CreateSessionRequest{}, err
	}
	jitApprovers, err := nonEmptyStringList(d, "jit_approvers", "email")
	if err != nil {
		return adaptive.CreateSessionRequest{}, err
	}
	groups, err := nonEmptyStringList(d, "groups", "group name")
	if err != nil {
		return adaptive.CreateSessionRequest{}, err
	}

	sType := d.Get("type").(string)
	wireType, valid := getSessionType(sType)
	if !valid {
		return adaptive.CreateSessionRequest{}, fmt.Errorf("invalid session type: %s", sType)
	}

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return adaptive.CreateSessionRequest{}, err
	}

	req := adaptive.CreateSessionRequest{
		SessionName:       sName,
		ResourceName:      d.Get("resource").(string),
		AuthorizationName: d.Get("authorization").(string),
		ClusterName:       d.Get("cluster").(string),
		SessionTTL:        d.Get("ttl").(string),
		SessionType:       wireType,
		SessionUsers:      userEmails,
		IsJITEnabled:      d.Get("is_jit_enabled").(bool),
		AccessApprovers:   jitApprovers,
		PauseTimeout:      d.Get("pause_timeout").(string),
		Memory:            d.Get("memory").(string),
		CPU:               d.Get("cpu").(string),
		UsersTags:         userTags,
		Groups:            groups,
		IdleTimeout:       d.Get("idle_timeout").(string),
		ScriptOnlyAccess:  d.Get("script_only_access").(bool),

		DisableOutputCapture: d.Get("disable_output_capture").(bool),
		DisableDataStudio:    d.Get("disable_data_studio").(bool),
		DisableWebCLI:        d.Get("disable_web_cli").(bool),
	}
	// Empty mode is a backend preserve operation, not an explicit clear. Never
	// resend a mode merely because Read observed it in state.
	if configHasAttr(d, "jit_mode") {
		mode := d.GetRawConfig().GetAttr("jit_mode").AsString()
		if mode != "" {
			req.JITMode = d.Get("jit_mode").(string)
		}
	}

	if configHasAttr(d, "auto_approval") {
		v := d.Get("auto_approval").(bool)
		req.AutoApproval = &v
	}
	if configHasAttr(d, "jit_multi_approver") {
		v := d.Get("jit_multi_approver").(bool)
		req.JITMultiApprover = &v
	}
	if configHasAttr(d, "jit_total_approvers") {
		v := d.Get("jit_total_approvers").(int)
		req.JITTotalApprovers = &v
	}
	if configHasAttr(d, "max_jit_request_duration") {
		v := d.Get("max_jit_request_duration").(string)
		write := true
		if d.GetRawConfig().GetAttr("max_jit_request_duration").AsString() == "" {
			plan := d.GetRawPlan()
			if !plan.IsNull() && plan.IsKnown() && plan.Type().IsObjectType() && plan.Type().HasAttribute("max_jit_request_duration") {
				p := plan.GetAttr("max_jit_request_duration")
				if p.IsKnown() && (p.IsNull() || p.AsString() != "") {
					write = false // A retained plan value is not permission to clear.
				} else {
					// SDKv2 marks a computed empty string unknown and Get can
					// return prior state. Resolve the authorized apply-time clear.
					v = ""
				}
			} else if v != "" {
				return adaptive.CreateSessionRequest{}, fmt.Errorf("cannot safely resolve an endpoint duration clear without its Terraform plan")
			}
		}
		if write {
			req.MaxJITRequestDuration = &v
		}
	}

	return req, nil
}

// nonEmptyStringList reads a TypeList of strings, rejecting empty entries.
// what names the kind of element for error messages ("email", "group name").
func nonEmptyStringList(d *schema.ResourceData, key, what string) ([]string, error) {
	raw, ok := d.Get(key).([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a list of strings", key)
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a string", what)
		}
		if s == "" {
			return nil, fmt.Errorf("%s cannot be empty", what)
		}
		out = append(out, s)
	}
	return out, nil
}

// setEndpointOutputs copies the server-computed fields onto the state.
func setEndpointOutputs(d *schema.ResourceData, r *adaptive.SessionReadResponse) error {
	if err := d.Set("status", r.Status); err != nil {
		return err
	}
	if err := d.Set("exposed", r.Exposed); err != nil {
		return err
	}
	if err := d.Set("expose_type", r.ExposeType); err != nil {
		return err
	}
	return d.Set("expose_status", r.ExposeStatus)
}

func ResourceAdaptiveSessionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	req, err := sessionRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateSession(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(resp.ID)

	// Best-effort: surface the server-computed outputs right away. A failure
	// here is not fatal — the next refresh picks them up.
	if r, rerr := client.ReadSession(ctx, resp.ID); rerr == nil && r != nil {
		if err := setEndpointOutputs(d, r); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func ResourceAdaptiveSessionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	r, err := client.ReadSession(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if r == nil {
		return notFoundGone(d)
	}

	if err := validateEndpointPolicyRead(r); err != nil {
		return diag.FromErr(err)
	}

	// Optional+Computed policy always observes endpoint-local values. Raw
	// configuration, never observed state, controls pointer-field write ownership.

	isImport := isImportRead(d, "name")
	if missingOnImport(d, isImport, r.Name) {
		return nil
	}

	if err := d.Set("name", r.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("resource", r.Resource); err != nil {
		return diag.FromErr(err)
	}

	// "direct" (and unset) are spellings of the wire type "cli", so keep the
	// user's spelling whenever it resolves to what the server reports.
	if wire, ok := getSessionType(d.Get("type").(string)); isImport || !ok || wire != r.SessionType {
		if err := d.Set("type", r.SessionType); err != nil {
			return diag.FromErr(err)
		}
	}

	// cluster, memory and cpu carry workspace defaults when left unset, so like
	// the other optional strings they are only refreshed once state holds a
	// value of their own.
	for _, f := range []struct {
		key    string
		server string
	}{
		{"ttl", r.TTL},
		{"authorization", r.Authorization},
		{"pause_timeout", r.PauseTimeout},
		{"cluster", r.Cluster},
		{"memory", r.Memory},
		{"cpu", r.CPU},
	} {
		if isImport || d.Get(f.key).(string) != "" {
			if err := d.Set(f.key, f.server); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	// The server normalizes an unset idle timeout to its default "15m"; an
	// unset attribute reading back the default is not drift.
	idle := d.Get("idle_timeout").(string)
	if isImport || (idle != "" && !(idle == "15m" && r.IdleTimeout == "")) {
		if err := d.Set("idle_timeout", r.IdleTimeout); err != nil {
			return diag.FromErr(err)
		}
	}

	for _, f := range []struct {
		key    string
		server []string
	}{
		{"users", r.SessionUsers},
		{"groups", r.Groups},
		{"tags", r.UserTags},
		{"jit_approvers", r.AccessApprovers},
	} {
		if err := setStringSet(d, f.key, f.server); err != nil {
			return diag.FromErr(err)
		}
	}

	// Observe endpoint-local policy in both directions, including false and zero.
	for _, f := range []struct {
		key, wire string
		server    bool
	}{
		{"is_jit_enabled", "isJitEnabled", r.IsJITEnabled},
		{"script_only_access", "scriptOnlyAccess", r.ScriptOnlyAccess},
		{"disable_output_capture", "disableOutputCapture", r.DisableOutputCapture},
		{"disable_data_studio", "disableDataStudio", r.DisableDataStudio},
		{"disable_web_cli", "disableWebCli", r.DisableWebCLI},
		{"auto_approval", "autoApproval", r.AutoApproval},
		{"jit_multi_approver", "jitMultiApprover", r.JITMultiApprover},
	} {
		if !r.HasField(f.wire) {
			continue
		}
		if err := d.Set(f.key, f.server); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("jit_mode", r.JITMode); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("max_jit_request_duration", r.MaxJITRequestDuration); err != nil {
		return diag.FromErr(err)
	}
	if r.HasField("jitTotalApprovers") {
		if err := d.Set("jit_total_approvers", r.JITTotalApprovers); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := setEndpointOutputs(d, r); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func ResourceAdaptiveSessionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	oldType, newType := d.GetChange("type")
	if err := validateEndpointTypeChange(oldType.(string), newType.(string)); err != nil {
		return diag.FromErr(err)
	}
	client := m.(*adaptive.Client)

	req, err := sessionRequestFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	live, err := client.ReadSession(ctx, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("cannot verify endpoint policy before updating: %w", err))
	}
	if err := validateEndpointPolicyRead(live); err != nil {
		return diag.FromErr(err)
	}
	if live.SessionType != req.SessionType {
		return diag.Errorf("endpoint type cannot be changed in place; create a separate endpoint and migrate explicitly")
	}

	resp, err := client.UpdateSession(ctx, d.Id(), req)
	if err != nil {
		return diag.FromErr(err)
	}
	if resp != nil && resp.ID != "" && resp.ID != d.Id() {
		return diag.Errorf("endpoint update response identifies a different object; original tracked ID retained")
	}

	// Policy writes can partially fail despite a successful response. Read the
	// exact original target and confirm the requested policy before state writes.
	r, err := client.ReadSession(ctx, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("endpoint may have updated but read-back failed; refresh before retrying: %w", err))
	}
	if err := verifyEndpointPolicyWrite(req, r); err != nil {
		return diag.FromErr(err)
	}
	if req.MaxJITRequestDuration != nil {
		if err := d.Set("max_jit_request_duration", r.MaxJITRequestDuration); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := setEndpointOutputs(d, r); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// configHasAttr reports whether key is set in the user's configuration.
// Terraform only exposes the raw configuration during plan and apply, so this
// is false during refresh. Observed state never establishes write ownership.
func configHasAttr(d *schema.ResourceData, key string) bool {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute(key) {
		return false
	}
	v := raw.GetAttr(key)
	return !v.IsNull() && v.IsKnown()
}

// setStringSet writes a list attribute whose ordering carries no meaning,
// keeping the stored ordering while the two agree as sets.
func setStringSet(d *schema.ResourceData, key string, server []string) error {
	return d.Set(key, setPreservingOrder(stateStringSlice(d, key), server))
}

func ResourceAdaptiveSessionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sessionID := d.Id()
	client := m.(*adaptive.Client)

	_, err := client.DeleteSession(ctx, sessionID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
