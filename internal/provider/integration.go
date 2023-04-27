package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

var (
	validIntegrationTypes = []string{
		"aws",
		"azure",
		"cockroachdb",
		"gcp",
		"google",
		"mongodb",
		"mysql",
		"okta",
		"postgres",
		"services",
		"ssh",
		"kubernetes",
	}
)

func isValidIntegrationType(t string) bool {
	for _, v := range validIntegrationTypes {
		if v == t {
			return true
		}
	}
	return false
}

// TODO: Add generic attributes like:
// - Authorization

func resourceAdaptiveResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveResourceCreate,
		ReadContext:   resourceAdaptiveResourceRead,
		UpdateContext: resourceAdaptiveResourceUpdate,
		DeleteContext: resourceAdaptiveResourceDelete,

		Schema: map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Type of the adaptive integration.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the adaptive integration.",
			},
			"uri": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "URI of the adaptive integration.",
			},
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Hostname of the adaptive integration.",
			},
			"port": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Port number of the adaptive integration.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for the adaptive integration authentication.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Password for the adaptive integration authentication.",
			},
			"database_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "",
			},
			"ssl_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "",
			},
			"api_server": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by Kubernetes",
			},
			"cluster_token": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by Kubernetes",
			},
			"cluster_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by Kubernetes",
			},
			"root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by CockroachDB",
			},
			"region_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"access_key_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"secret_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"application_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"hostname": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"project_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"urls": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
			"key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used by AWS",
			},
		},
	}
}

// Returns a YAML marshallable struct for the integration configuration
func schemaToResourceIntegrationConfiguration(d *schema.ResourceData, intType string) (any, error) {
	switch intType {
	case "aws":
		return schemaToAWSIntegrationConfiguration(d), nil
	case "azure":
		return schemaToAzureIntegrationConfiguration(d), nil
	case "cockroachdb":
		return schemaToCockroachDBIntegrationConfiguration(d), nil
	case "gcp":
		return schemaToGCPIntegrationConfiguration(d), nil
	case "google":
		return schemaToGoogleOAuthIntegrationConfiguration(d), nil
	case "mongodb":
		return schemaToMongoIntegrationConfiguration(d), nil
	case "mysql":
		return schemaToMySQLIntegrationConfiguration(d), nil
	case "okta":
		return schemaToOktaIntegrationConfiguration(d), nil
	case "postgres":
		return schemaToPostgresIntegrationConfiguration(d), nil
	case "services":
		return schemaToServiceListIntegrationConfiguration(d), nil
	case "ssh":
		return schemaToSSHIntegrationConfiguration(d), nil
	case "kubernetes":
		return schemaToKubernetesIntegrationConfiguration(d), nil
	default:
		return nil, fmt.Errorf("invalid adaptive resource type %s", intType)
	}

}

func resourceAdaptiveResourceCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	iType := d.Get("type").(string)
	if !isValidIntegrationType(iType) {
		return diag.FromErr(fmt.Errorf("invalid integration type %s", iType))
	}
	obj, err := schemaToResourceIntegrationConfiguration(d, iType)
	if err != nil {
		return diag.FromErr(err)
	}

	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("could not marshal resource configuration %w", err))
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	if iType == "services" {
		iType = "servicelist"
	}
	resp, err := client.CreateResource(ctx, rName, iType, config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveResourceRead(ctx, d, m)
	return nil
}

func resourceAdaptiveResourceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveResourceUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	iType := d.Get("type").(string)
	if !isValidIntegrationType(iType) {
		return diag.FromErr(fmt.Errorf("invalid integration type %s", iType))
	}
	obj, err := schemaToResourceIntegrationConfiguration(d, iType)
	if err != nil {
		return diag.FromErr(err)
	}

	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	if iType == "services" {
		iType = "servicelist"
	}
	_, err = client.UpdateResource(resourceID, iType, config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveResourceRead(ctx, d, m)
}

func resourceAdaptiveResourceDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
