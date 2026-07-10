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
				Type:        schema.TypeString,
				Default:     SessionTypeDefault,
				Optional:    true,
				Description: "The type of session to create.",
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
				Optional:    true,
				Default:     false,
				Elem:        &schema.Schema{Type: schema.TypeBool},
				Description: "Whether Just-In-Time access is enabled for the session",
			},
			"jit_approvers": {
				Type:     schema.TypeList,
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
				Optional:    true,
				Default:     false,
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
			"last_updated": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The last time the session was updated.",
			},
		},
	}
}

func isValidSessionType(t string) bool {
	return t == SessionTypeDirect || t == SessionTypeClient || t == SessionTypeScript || t == SessionTypeCLI || t == SessionTypeServices
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
		case SessionTypeServices:
			return "services", true
		default:
			return "", false
		}
	}

	return "", false

}

func ResourceAdaptiveSessionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	sName, err := integrations.NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	users := d.Get("users").([]interface{})
	userEmails := make([]string, len(users))
	for i, u := range users {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			userEmails[i] = val
		}
	}

	sType := d.Get("type").(string)
	validSessionType, valid := getSessionType(sType)
	if !valid {
		return diag.Errorf("Invalid session type: %s", sType)
	}

	isJitEnabled := d.Get("is_jit_enabled")
	if _, ok := isJitEnabled.(bool); !ok {
		return diag.FromErr(fmt.Errorf("is_jit_enabled must be a boolean"))
	}

	jitApprovers := d.Get("jit_approvers")
	if jitApprovers == nil {
		jitApprovers = []string{}
	}
	if _, ok := jitApprovers.([]interface{}); !ok {
		return diag.FromErr(fmt.Errorf("jit_approvers must be a list of strings"))
	}
	jitApproversEmails := make([]string, len(jitApprovers.([]interface{})))
	for i, u := range jitApprovers.([]interface{}) {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			jitApproversEmails[i] = val
		}
	}

	pauseTimeout := d.Get("pause_timeout")
	if _, ok := pauseTimeout.(string); !ok {
		return diag.FromErr(fmt.Errorf("pause_timeout must be a string"))
	}

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	mem := d.Get("memory")
	if _, ok := mem.(string); !ok {
		return diag.FromErr(fmt.Errorf("memory must be a string"))
	}

	cpu := d.Get("cpu")
	if _, ok := cpu.(string); !ok {
		return diag.FromErr(fmt.Errorf("cpu must be a string"))
	}

	idleTimeout := d.Get("idle_timeout")
	if _, ok := idleTimeout.(string); !ok {
		return diag.FromErr(fmt.Errorf("idle_timeout must be a string"))
	}

	groups := d.Get("groups")
	groupsVal := make([]string, 0)

	if groups != nil {
		if groupsList, ok := groups.([]interface{}); ok {
			for _, u := range groupsList {
				if val, ok := u.(string); !ok {
					return diag.FromErr(fmt.Errorf("group name must be a string"))
				} else {
					if len(val) == 0 {
						return diag.FromErr(fmt.Errorf("group name cannot be empty"))
					}
					groupsVal = append(groupsVal, val)
				}
			}
		}
	}

	scriptOnlyAccess := d.Get("script_only_access").(bool)

	resp, err := client.CreateSession(
		ctx,
		sName,
		d.Get("resource").(string),
		d.Get("authorization").(string),
		d.Get("cluster").(string),
		d.Get("ttl").(string),
		validSessionType,
		isJitEnabled.(bool),
		jitApproversEmails,
		pauseTimeout.(string),
		userEmails,
		mem.(string), cpu.(string),
		userTags,
		groupsVal,
		idleTimeout.(string),
		scriptOnlyAccess,
	)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)

	// stateConf := &resource.StateChangeConf{
	// 	Pending:    []string{"creating", "backing-up", "modifying"},
	// 	Target:     []string{"available"},
	// 	Refresh:    resourceAwsRDSClusterStateRefreshFunc(d, meta),
	// 	Timeout:    120 * time.Minute,
	// 	MinTimeout: 3 * time.Second,
	// }

	// // Wait, catching any errors
	// _, err := stateConf.WaitForState()
	// if err != nil {
	// 	return fmt.Errorf("[WARN] Error waiting for RDS Cluster state to be \"available\": %s", err)
	// }

	ResourceAdaptiveSessionRead(ctx, d, m)
	return nil
}

func ResourceAdaptiveSessionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}

func ResourceAdaptiveSessionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	sessionID := d.Id()

	// if d.HasChange("type") {
	// 	return diag.Errorf("Cannot change type after creation")
	// }
	// if d.HasChange("resource") {
	// 	return diag.Errorf("Cannot change resource after creation")
	// }
	// if d.HasChange("authorization") {
	// 	return diag.Errorf("Cannot change authorizaton after creation")
	// }
	// if d.HasChange("cluster") {
	// 	return diag.Errorf("Cannot change cluster after creation")
	// }
	// if d.HasChange("ttl") {
	// 	return diag.Errorf("Cannot change ttl after creation")
	// }
	// if d.HasChange("memory") {
	// 	return diag.Errorf("Cannot change memory after creation")
	// }
	// if d.HasChange("cpu") {
	// 	return diag.Errorf("Cannot change cpu after creation")
	// }

	users := d.Get("users").([]interface{})
	userEmails := make([]string, len(users))
	if len(users) == 0 {
		var diags diag.Diagnostics
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "No endpoint users provided",
			Detail:   "Since no endpoint users were provided, the creator of the session will be the only user.",
		})
	}
	for i, u := range users {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			userEmails[i] = val
		}

	}

	seshType := d.Get("type").(string)
	validSessionType, valid := getSessionType(seshType)
	if !valid {
		return diag.Errorf("Invalid session type: %s", seshType)
	}

	isJitEnabled := d.Get("is_jit_enabled")
	if _, ok := isJitEnabled.(bool); !ok {
		return diag.FromErr(fmt.Errorf("is_jit_enabled must be a boolean"))
	}

	jitApprovers := d.Get("jit_approvers")
	if jitApprovers == nil {
		jitApprovers = []string{}
	}
	if _, ok := jitApprovers.([]interface{}); !ok {
		return diag.FromErr(fmt.Errorf("jit_approvers must be a list of strings"))
	}
	jitApproversEmails := make([]string, len(jitApprovers.([]interface{})))
	for i, u := range jitApprovers.([]interface{}) {
		if val, ok := u.(string); !ok {
			return diag.FromErr(fmt.Errorf("email must be a string"))
		} else {
			if len(val) == 0 {
				return diag.FromErr(fmt.Errorf("email cannot be empty"))
			}
			jitApproversEmails[i] = val
		}
	}

	pauseTimeout := d.Get("pause_timeout")
	if _, ok := pauseTimeout.(string); !ok {
		return diag.FromErr(fmt.Errorf("pause_timeout must be a string"))
	}

	idleTimeout := d.Get("idle_timeout")
	if _, ok := idleTimeout.(string); !ok {
		return diag.FromErr(fmt.Errorf("idle_timeout must be a string"))
	}

	mem := d.Get("memory")
	if _, ok := mem.(string); !ok {
		return diag.FromErr(fmt.Errorf("memory must be a string"))
	}

	cpu := d.Get("cpu")
	if _, ok := cpu.(string); !ok {
		return diag.FromErr(fmt.Errorf("cpu must be a string"))
	}

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	groups := d.Get("groups")
	groupsVal := make([]string, 0)

	if groups != nil {
		if groupsList, ok := groups.([]interface{}); ok {
			for _, u := range groupsList {
				if val, ok := u.(string); !ok {
					return diag.FromErr(fmt.Errorf("group name must be a string"))
				} else {
					if len(val) == 0 {
						return diag.FromErr(fmt.Errorf("group name cannot be empty"))
					}
					groupsVal = append(groupsVal, val)
				}
			}
		}
	}

	// Get authorization - use GetOk to preserve empty values from state
	authorizationName := ""
	if auth, ok := d.GetOk("authorization"); ok {
		authorizationName = auth.(string)
	}

	scriptOnlyAccess := d.Get("script_only_access").(bool)

	resp, err := client.UpdateSession(
		ctx,
		sessionID,
		d.Get("name").(string),
		d.Get("resource").(string),
		authorizationName,
		d.Get("cluster").(string),
		d.Get("ttl").(string),
		validSessionType,
		isJitEnabled.(bool),
		jitApproversEmails,
		pauseTimeout.(string),
		userEmails,
		mem.(string), cpu.(string),
		userTags,
		groupsVal,
		idleTimeout.(string),
		scriptOnlyAccess,
	)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	ResourceAdaptiveSessionRead(ctx, d, m)
	return nil
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
