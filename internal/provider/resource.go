package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
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
		"mongodb_aws_secrets_manager",
		"mysql",
		"mysql_aws_secrets_manager",
		"okta",
		"postgres",
		"postgres_aws_secrets_manager",
		"services",
		"serverlist",
		"ssh",
		"kubernetes",
		"awsdocumentdb",
		"awsredshift",
		"zerotier",
		"rdp_windows",
		"mongodb_atlas",
		"awssecretsmanager",

		//new types to be added later
		"sql_server",
		"azuresqlserver",
		"splunk",
		"datadog",
		"sqlserver_aws_secrets_manager",
		"coralogix",
		"jumpcloud",
		"msteams",
		"yugabytedb",
		"onelogin",
		"elasticsearch",
		"paloalto_ngfw",
		"fortinet_ngfw",
		"cisco_ngfw",
		"snowflake",
		"snowflake_aws_secrets_manager",
		"custom_siem_webhook",
		"aruba_sw",
		"aruba_instant_on",
		"hpe_switch",
		"syslog",
		"customintegration",
		"clickhouse",
		"keyspaces",
		"rabbitmq",
		"azurecosmosnosql",
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
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Type of the Adaptive resource",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Adaptive resource",
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Optional tags",
			},
			"uri": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Connection string to a resource. Used by MongoDB",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace where pods will be created. Used by Kubernetes resource",
			},
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources",
			},
			"hostname": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources",
			},
			"port": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Port number of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for the adaptive resource authentication. Used by CockroachDB, Postgres, Mysql, SSH resources",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Password for the adaptive integration authentication.Used by CockroachDB, Postgres, Mysql, SSH resources",
			},
			"database_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The name of the database to connect to. Used by CockroachDB, Postgres, Mysql resources",
			},
			"root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The root certificate to use for the CockroachDB instance.",
			},
			"ssl_mode": {
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					if i == "" {
						return nil, nil
					}
					validValues := []string{
						"prefer", "allow", "require", "verify-ca", "verify-full", "disable",
					}

					if !slices.Contains(validValues, i.(string)) {
						return nil, []error{fmt.Errorf("invalid value for ssl_mode: %s", i)}
					}

					return nil, nil
				},
				Description: "The SSL mode to use when connecting to the database. Used by CockroachDB, Postgres, Mysql resources",
			},
			"api_server": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The url for Kubernetes API server. Used by Kubernetes resource",
			},
			"cluster_token": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The cluster token for Kubernetes API server. Used by Kubernetes resource",
			},
			"cluster_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The cluster token for Kubernetes API server. Used by Kubernetes resource",
			},
			"region_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region name. Used by AWS resource.",
			},
			"access_key_id": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The AWS access key id. Used by AWS resource.",
			},
			"secret_access_key": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The AWS secret access key in plaintext. Used by AWS resource.",
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The Azure tenant ID. Used by Azure resource.",
			},
			"application_id": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The Azure application ID. Used by Azure resource.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The client secret for a resource. Used by Azure, Google, Okta resources.",
			},
			"api_client_id": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The API client ID for a resource. Used by Azure resource.",
			},
			"api_client_secret": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The API client secret for a resource. Used by Azure resource.",
			},
			"login_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The login URL for a resource",
			},
			"warehouse": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Snowflake warehouse name. Used by Snowflake resource",
			},
			"schema": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Snowflake schema name. Used by Snowflake resource",
			},
			"clientcert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Snowflake client certificate. Used by Snowflake resource",
			},
			"role": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Snowflake role name. Used by Snowflake resource",
			},
			"protocol": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The protocol to use when connecting to the resource",
			},
			"project_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The GCP project ID. Used by GCP resource",
			},
			"key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The content of GCP key file. Used by GCP resource",
			},
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The domain name for a resource. Used by Google, Okta resource",
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The client ID of a OAuth application. Used by Google, Okta resource",
			},
			"urls": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Comma-separated list of URLs. Used by Services resource",
			},
			"hosts": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of hosts. Used by Services resource",
			},
			"default_user": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Default user for the Services resource",
			},

			"key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The SSH key to use when connecting to the instance. If not specified, password authentication will be used. Used by SSH resource",
			},
			"private_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "",
			},
			"public_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "",
			},
			"organization_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "",
			},
			"arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret",
			},
			"region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region of the AWS Secrets Manager secret",
			},
			"secret_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS Secrets Manager secret ID",
			},
			"aws_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret",
			},
			"aws_region_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region of the AWS Secrets Manager secret",
			},
			"api_token": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The API token for the service",
			},

			"private_key": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The private key for the service",
			},
			"application_name": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The application name for the service",
			},
			"sub_system_name": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The sub system name for the service",
			},
			"shared_secret": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The shared secret for the integration",
			},
			"image": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The Docker image to use for the YugabyteDB resource",
			},
			"service_account_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The service account name to use for the YugabyteDB resource",
			},
			"dd_site": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The Datadog site to send data to",
			},
			"dd_api_key": {
				Type:        schema.TypeString,
				Optional:    false,
				Description: "The Datadog API key",
			},
			"index": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Elasticsearch index to send data to",
			},
			"use_proxy": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to use proxy",
			},
			"webui_port": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The web UI port",
			},
			"use_service_account": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to use service account for authentication. Used by GCP resource",
			},
			"create_if_not_exists": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to create the Keyspaces keyspace if it does not exist",
			},
			"network_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The network ID for ZeroTier network",
			},
			"tls_root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The root certificate to use for the Postgres-like resources.",
			},
			"tls_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The certificate file to use for the Postgres-like resources.",
			},
			"tls_key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The key file to use for the Postgres-like resources.",
			},
		},
	}
}

// Returns a YAML marshallable struct for the integration configuration
func schemaToResourceIntegrationConfiguration(d *schema.ResourceData, intType string) (any, error) {
	switch intType {
	case "aws":
		return schemaToAWSIntegrationConfiguration(d), nil
	case "awsredshift":
		return schemaToAWSRedshiftIntegrationConfiguration(d), nil
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
	case "mongodb_aws_secrets_manager":
		return schemaToMongoAWSIntegrationConfiguration(d), nil
	case "mysql":
		return schemaToMySQLIntegrationConfiguration(d), nil
	case "mysql_aws_secrets_manager":
		return schemaToMySQLAWSIntegrationConfiguration(d), nil
	case "okta":
		return schemaToOktaIntegrationConfiguration(d), nil
	case "postgres":
		return schemaToPostgresIntegrationConfiguration(d), nil
	case "postgres_aws_secrets_manager":
		return schemaToPostgresAWSIntegrationConfiguration(d), nil
	case "services":
		return schemaToServiceListIntegrationConfiguration(d), nil
	case "serverlist":
		return schemaToServerListIntegrationConfiguration(d)
	case "ssh":
		return schemaToSSHIntegrationConfiguration(d), nil
	case "kubernetes":
		return schemaToKubernetesIntegrationConfiguration(d), nil
	case "awsdocumentdb":
		return schemaToAWSDocumentDBIntegrationConfiguration(d), nil
	case "zerotier":
		return schemaToZeroTierIntegrationConfiguration(d), nil
	case "mongodb_atlas":
		return schemaToMongoAtlasIntegrationConfiguration(d), nil
	case "rdp_windows":
		return schemaToRDPWindowsIntegrationConfiguration(d), nil
	case "awssecretsmanager":
		return schemaToAWSSecretsManagerConfiguration(d), nil
	case "sql_server":
		return schemaToSQLServerIntegrationConfiguration(d), nil
	case "azuresqlserver":
		return schemaToAzureSQLServerIntegrationConfiguration(d), nil
	case "splunk":
		return schemaToSplunkIntegrationConfiguration(d), nil
	case "datadog":
		return schemaToDatadogIntegrationConfiguration(d), nil
	case "sqlserver_aws_secrets_manager":
		return schemaToSQLServerAWSIntegrationConfiguration(d), nil
	case "coralogix":
		return schemaToCoralogixIntegrationConfiguration(d), nil
	case "jumpcloud":
		return schemaToJumpCloudIntegrationConfiguration(d), nil
	case "msteams":
		return schemaToMSTeamsIntegrationConfiguration(d), nil
	case "yugabytedb":
		return schemaToYugabyteDBIntegrationConfiguration(d), nil
	case "onelogin":
		return schemaToOneLoginIntegrationConfiguration(d), nil
	case "elasticsearch":
		return schemaToElasticsearchIntegrationConfiguration(d), nil
	case "paloalto_ngfw":
		return schemaToPaloAltoNGFWIntegrationConfiguration(d), nil
	case "fortinet_ngfw":
		return schemaToFortinetNGFWIntegrationConfiguration(d), nil
	case "cisco_ngfw":
		return schemaToCiscoNGFWIntegrationConfiguration(d), nil
	case "snowflake":
		return schemaToSnowflakeIntegrationConfiguration(d), nil
	case "snowflake_aws_secrets_manager":
		return schemaToSnowflakeAWSIntegrationConfiguration(d), nil
	case "custom_siem_webhook":
		return schemaToCustomSIEMWebhookIntegrationConfiguration(d), nil
	case "aruba_sw":
		return schemaToArubaSWIntegrationConfiguration(d), nil
	case "aruba_instant_on":
		return schemaToArubaInstantOnIntegrationConfiguration(d), nil
	case "hpe_switch":
		return schemaToHPESwitchIntegrationConfiguration(d), nil
	case "syslog":
		return schemaToSyslogIntegrationConfiguration(d), nil
	case "customintegration":
		return schemaToCustomIntegrationConfiguration(d), nil
	case "clickhouse":
		return schemaToClickHouseIntegrationConfiguration(d), nil
	case "keyspaces":
		return schemaToKeyspacesIntegrationConfiguration(d), nil
	case "rabbitmq":
		return schemaToRabbitMQIntegrationConfiguration(d), nil
	case "azurecosmosnosql":
		return schemaToAzureCosmosNoSQLIntegrationConfiguration(d), nil
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

	userTags, err := tagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateResource(ctx, rName, iType, config, userTags)
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

	userTags, err := tagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, iType, config, userTags)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveResourceRead(ctx, d, m)
}

func resourceAdaptiveResourceDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
