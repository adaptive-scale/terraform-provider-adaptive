package components

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
)

var (
	validIntegrationTypes = []string{
		"aws",
		"azure",
		"azureactivedirectory",
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

// validateIntegrationType returns a schema validation function that checks
// if the provided value is a valid integration type.
func validateIntegrationType(i any, p cty.Path) diag.Diagnostics {
	v, ok := i.(string)
	if !ok {
		return diag.Errorf("expected type to be string")
	}

	if !isValidIntegrationType(v) {
		return diag.Errorf("invalid integration type %q; valid types are: %s", v, strings.Join(validIntegrationTypes, ", "))
	}

	return nil
}

// TODO: Add generic attributes like:
// - Authorization

func ResourceAdaptiveResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: ResourceAdaptiveResourceCreate,
		ReadContext:   ResourceAdaptiveResourceRead,
		UpdateContext: ResourceAdaptiveResourceUpdate,
		DeleteContext: ResourceAdaptiveResourceDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"type": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "Type of the Adaptive resource",
				ValidateDiagFunc: validateIntegrationType,
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
			"tolerations": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The tolerations configuration in YAML format. Used by Kubernetes resource",
			},
			"annotations": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The annotations configuration in YAML format. Used by Kubernetes resource",
			},
			"node_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The node selector configuration in YAML format. Used by Kubernetes resource",
			},
			"node_affinity": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The node affinity configuration in YAML format. Used by Kubernetes resource",
			},
			"region_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region name. Used by AWS resource.",
			},
			"access_key_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS access key id. Used by AWS resource.",
			},
			"secret_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS secret access key in plaintext. Used by AWS resource.",
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure tenant ID. Used by Azure resource.",
			},
			"application_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Azure application ID. Used by Azure resource.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The client secret for a resource. Used by Azure, Google, Okta resources.",
			},
			"api_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API client ID for a resource. Used by Azure resource.",
			},
			"api_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
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
				Optional:    true,
				Description: "The API token for the service",
			},

			"private_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The private key for the service",
			},
			"application_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The application name for the service",
			},
			"sub_system_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The sub system name for the service",
			},
			"shared_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The shared secret for the integration",
			},
			"image": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Docker image to use for the YugabyteDB resource",
			},
			"service_account_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The service account name to use for the YugabyteDB resource",
			},
			"dd_site": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The Datadog site to send data to",
			},
			"dd_api_key": {
				Type:        schema.TypeString,
				Optional:    true,
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
			"token_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The token ID for the service",
			},
			"url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The URL for the service",
			},
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The API key",
			},
			"app_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The app ID",
			},
			"app_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The app key",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The version",
			},
			"database_account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database account",
			},
			"database_username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database username",
			},
			"database_password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database password",
			},
			"default_cluster": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The default cluster",
			},
		},
	}
}

// Returns a YAML marshallable struct for the integration configuration
func schemaToResourceIntegrationConfiguration(d *schema.ResourceData, intType string) (any, error) {
	switch intType {
	case "aws":
		return integrations.SchemaToAWSIntegrationConfiguration(d), nil
	case "awsredshift":
		return integrations.SchemaToAWSRedshiftIntegrationConfiguration(d), nil
	case "azure":
		return integrations.SchemaToAzureIntegrationConfiguration(d), nil
	case "cockroachdb":
		return integrations.SchemaToCockroachDBIntegrationConfiguration(d), nil
	case "gcp":
		return integrations.SchemaToGCPIntegrationConfiguration(d), nil
	case "google":
		return integrations.SchemaToGoogleOAuthIntegrationConfiguration(d), nil
	case "mongodb":
		return integrations.SchemaToMongoIntegrationConfiguration(d), nil
	case "mongodb_aws_secrets_manager":
		return integrations.SchemaToMongoAWSIntegrationConfiguration(d), nil
	case "mysql":
		return integrations.SchemaToMySQLIntegrationConfiguration(d), nil
	case "mysql_aws_secrets_manager":
		return integrations.SchemaToMySQLAWSIntegrationConfiguration(d), nil
	case "okta":
		return integrations.SchemaToOktaIntegrationConfiguration(d), nil
	case "postgres":
		return integrations.SchemaToPostgresIntegrationConfiguration(d), nil
	case "postgres_aws_secrets_manager":
		return integrations.SchemaToPostgresAWSIntegrationConfiguration(d), nil
	case "services":
		return integrations.SchemaToServiceListIntegrationConfiguration(d), nil
	case "serverlist":
		return integrations.SchemaToServerListIntegrationConfiguration(d)
	case "ssh":
		return integrations.SchemaToSSHIntegrationConfiguration(d), nil
	case "kubernetes":
		return integrations.SchemaToKubernetesIntegrationConfiguration(d), nil
	case "awsdocumentdb":
		return integrations.SchemaToAWSDocumentDBIntegrationConfiguration(d), nil
	case "zerotier":
		return integrations.SchemaToZeroTierIntegrationConfiguration(d), nil
	case "mongodb_atlas":
		return integrations.SchemaToMongoAtlasIntegrationConfiguration(d), nil
	case "rdp_windows":
		return integrations.SchemaToRDPWindowsIntegrationConfiguration(d), nil
	case "awssecretsmanager":
		return integrations.SchemaToAWSSecretsManagerConfiguration(d), nil
	case "sql_server":
		return integrations.SchemaToSQLServerIntegrationConfiguration(d), nil
	case "azuresqlserver":
		return integrations.SchemaToAzureSQLServerIntegrationConfiguration(d)
	case "splunk":
		return integrations.SchemaToSplunkIntegrationConfiguration(d), nil
	case "datadog":
		return integrations.SchemaToDatadogIntegrationConfiguration(d), nil
	case "sqlserver_aws_secrets_manager":
		return integrations.SchemaToSQLServerAWSIntegrationConfiguration(d), nil
	case "coralogix":
		return integrations.SchemaToCoralogixIntegrationConfiguration(d), nil
	case "jumpcloud":
		return integrations.SchemaToJumpCloudIntegrationConfiguration(d), nil
	case "msteams":
		return integrations.SchemaToMSTeamsIntegrationConfiguration(d), nil
	case "yugabytedb":
		return integrations.SchemaToYugabyteDBIntegrationConfiguration(d), nil
	case "onelogin":
		return integrations.SchemaToOneLoginIntegrationConfiguration(d), nil
	case "elasticsearch":
		return integrations.SchemaToElasticsearchIntegrationConfiguration(d), nil
	case "paloalto_ngfw":
		return integrations.SchemaToPaloAltoNGFWIntegrationConfiguration(d), nil
	case "fortinet_ngfw":
		return integrations.SchemaToFortinetNGFWIntegrationConfiguration(d), nil
	case "cisco_ngfw":
		return integrations.SchemaToCiscoNGFWIntegrationConfiguration(d), nil
	case "snowflake":
		return integrations.SchemaToSnowflakeIntegrationConfiguration(d), nil
	case "snowflake_aws_secrets_manager":
		return integrations.SchemaToSnowflakeAWSIntegrationConfiguration(d), nil
	case "custom_siem_webhook":
		return integrations.SchemaToCustomSIEMWebhookIntegrationConfiguration(d), nil
	case "aruba_sw":
		return integrations.SchemaToArubaSWIntegrationConfiguration(d), nil
	case "aruba_instant_on":
		return integrations.SchemaToArubaInstantOnIntegrationConfiguration(d), nil
	case "hpe_switch":
		return integrations.SchemaToHPESwitchIntegrationConfiguration(d), nil
	case "syslog":
		return integrations.SchemaToSyslogIntegrationConfiguration(d), nil
	case "customintegration":
		return integrations.SchemaToCustomIntegrationConfiguration(d), nil
	case "clickhouse":
		return integrations.SchemaToClickHouseIntegrationConfiguration(d), nil
	case "keyspaces":
		return integrations.SchemaToKeyspacesIntegrationConfiguration(d), nil
	case "rabbitmq":
		return integrations.SchemaToRabbitMQIntegrationConfiguration(d), nil
	case "azurecosmosnosql":
		return integrations.SchemaToAzureCosmosNoSQLIntegrationConfiguration(d), nil
	default:
		return nil, fmt.Errorf("invalid adaptive resource type %s", intType)
	}
}

func ResourceAdaptiveResourceCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
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

	rName, err := integrations.NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	if iType == "services" {
		iType = "servicelist"
	}

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := integrations.DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateResource(ctx, rName, iType, config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	ResourceAdaptiveResourceRead(ctx, d, m)
	return nil
}

func ResourceAdaptiveResourceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func ResourceAdaptiveResourceUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
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

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := integrations.DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, iType, config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return ResourceAdaptiveResourceRead(ctx, d, m)
}

func ResourceAdaptiveResourceDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
