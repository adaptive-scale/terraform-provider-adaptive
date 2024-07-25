package provider

import (
	"context"
	"fmt"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
	SessionTTLOption180days = "180d"
	SessionTTLOption360days = "360d"
)

const (
	SessionTypeDefault = "direct"
	SessionTypeDirect  = "direct"
	SessionTypeScript  = "script"
	SessionTypeClient  = "client"
)

func resourceAdaptiveSession() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveSessionCreate,
		ReadContext:   resourceAdaptiveSessionRead,
		UpdateContext: resourceAdaptiveSessionUpdate,
		DeleteContext: resourceAdaptiveSessionDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Postgres database to create.",
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
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "3h",
				Description: "The port number of the Postgres instance to connect to.",
			},
			"authorization": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The authorization to use when creating the session.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Default:     "default",
				Optional:    true,
				Description: "The cluster in which this session should be created. If not provided will be set to default cluster set in workspace settings	of user's workspace",
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
				Description: "The list of users emails who can approve Just-In-Time access requests",
			},
			"pause_timeout": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "99999d",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The time after which the session will be paused if no user has connected to it. Defaults to never pause.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func isValidSessionType(t string) bool {
	return t == SessionTypeDirect || t == SessionTypeClient || t == SessionTypeScript
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
		default:
			return "", false
		}
	}

	return "", false

}

func resourceAdaptiveSessionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	sName, err := nameFromSchema(d)
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

	resourceAdaptiveSessionRead(ctx, d, m)
	return nil
}

func resourceAdaptiveSessionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}

func resourceAdaptiveSessionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	sessionID := d.Id()

	if d.HasChange("type") {
		return diag.Errorf("Cannot change type after creation")
	}
	if d.HasChange("resource") {
		return diag.Errorf("Cannot change resource after creation")
	}
	if d.HasChange("authorization") {
		return diag.Errorf("Cannot change authorizaton after creation")
	}
	if d.HasChange("cluster") {
		return diag.Errorf("Cannot change cluster after creation")
	}

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

	resp, err := client.UpdateSession(
		sessionID,
		d.Get("name").(string),
		d.Get("resource").(string),
		d.Get("authorization").(string),
		d.Get("cluster").(string),
		d.Get("ttl").(string),
		validSessionType,
		isJitEnabled.(bool),
		jitApproversEmails,
		pauseTimeout.(string),
		userEmails,
	)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveSessionRead(ctx, d, m)
	return nil
}

func resourceAdaptiveSessionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sessionID := d.Id()
	client := m.(*adaptive.Client)

	_, err := client.DeleteSession(sessionID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
