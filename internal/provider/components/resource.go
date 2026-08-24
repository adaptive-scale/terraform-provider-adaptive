package components

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
		"adaptive_rdp",
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
		"msteams_workflow",

		"1password",
		"adaptiveencrypt",
		"adaptiveremotedesktop",
		"avastbusinessedr",
		"awscloudwatchlogs",
		"awselasticcache",
		"awsdocumentdb_aws_secret_manager",
		"azure_documentdb",
		"big_query",
		"chrome",
		"cockroachdb_aws_secrets_manager",
		"confluent",
		"digitalocean",
		"fortinet_analyzer",
		"fortinet_manager",
		"heroku",
		"ivanti",
		"juniper_sw",
		"kafka",
		"ldap",
		"mongodb-do",
		"mongodb36",
		"oracle",
		"proxysql",
		"rdpldap",
		"redis",
		"securetunnels",
		"slack_webhook",
		"sophos_fw",
		"vnc",
		"windows-server-groups",
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
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
			"use_rds_iam": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Authenticate with short-lived AWS RDS IAM auth tokens instead of a stored password. The database user must be granted rds_iam, the RDS instance must have IAM authentication enabled, and `ssl_mode` cannot be `disable`. Used by Postgres resources.",
			},
			"use_irsa": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Mint RDS IAM tokens with `aws_service_account`, or with the platform's own IRSA / instance-profile identity when it is empty. Set to false to use `aws_access_key_id` / `aws_secret_access_key` instead. Only used when `use_rds_iam` is set.",
			},
			"aws_region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region used to mint RDS IAM tokens. Leave empty to auto-detect from an *.rds.amazonaws.com hostname. Only used when `use_rds_iam` is set.",
			},
			"aws_role_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "An IAM role to assume when minting RDS IAM tokens. With IRSA it overrides the service account's eks.amazonaws.com/role-arn annotation. Only used when `use_rds_iam` is set.",
			},
			"aws_access_key_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Static AWS access key id used to mint RDS IAM tokens. Requires `use_irsa = false`.",
			},
			"aws_secret_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Static AWS secret access key used to mint RDS IAM tokens. Requires `use_irsa = false`.",
			},
			"aws_service_account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of a Kubernetes ServiceAccount in the session cluster annotated with eks.amazonaws.com/role-arn; the platform mints RDS IAM tokens on its behalf. Requires `use_irsa = true`.",
			},
			"api_server": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The url for Kubernetes API server. Used by Kubernetes resource",
			},
			"cluster_token": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
				Description: "The API token for the service",
			},

			"private_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
				Description: "The key file to use for the Postgres-like resources.",
			},
			"token_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
				Description: "The database password",
			},
			"default_cluster": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The default cluster",
			},
			"use_tenant": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to use tenant for Azure Active Directory authentication",
			},
			"webhook_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Incoming webhook URL. Used by the msteams_workflow and slack_webhook resources.",
			},
			"service_account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "1Password service account token.",
			},
			"use_connect_server": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Reach 1Password through a self-hosted Connect server instead of the service account API.",
			},
			"connect_server_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "URL of the 1Password Connect server. Only used when `use_connect_server` is set.",
			},
			"value": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The value to encrypt. Used by the adaptiveencrypt resource.",
			},
			"cpu": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "CPU request for the remote desktop pod. Used by the adaptiveremotedesktop resource.",
			},
			"memory": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Memory request for the remote desktop pod. Used by the adaptiveremotedesktop resource.",
			},
			"storage": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Persistent storage size for the remote desktop pod. Used by the adaptiveremotedesktop resource.",
			},
			"log_group_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "CloudWatch log group to write to. Used by the awscloudwatchlogs resource.",
			},
			"log_stream_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "CloudWatch log stream to write to. Used by the awscloudwatchlogs resource.",
			},
			"tls_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Connect over TLS. Used by the redis, awselasticcache and awsdocumentdb_aws_secret_manager resources.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip verification of the server certificate. Used by the redis and awselasticcache resources.",
			},
			"use_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Connect over TLS. Used by the mongodb36 resource.",
			},
			"tls_ca_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "CA certificate used to verify the server. Used by the redis resource.",
			},
			"client_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client certificate presented to the server. Used by the proxysql resource.",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Private key for `client_cert`. Used by the proxysql resource.",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client certificate bundle. Used by the mongodb36 resource.",
			},
			"client_configuration": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Full client configuration (properties format). Used by the kafka resource.",
			},
			"access_control_method": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ElastiCache access-control method. Used by the awselasticcache resource.",
			},
			"access_control_group": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ElastiCache user-group id. Used by the awselasticcache resource.",
			},
			"credential_json": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Google service-account credentials JSON. Used by the big_query resource.",
			},
			"automation_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Browser automation mode. Used by the chrome resource.",
			},
			"fields": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Form fields to fill during automated login. Used by the chrome resource.",
			},
			"script": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Automation script to run in the browser. Used by the chrome resource.",
			},
			"prestart": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Commands to run before the browser starts. Used by the chrome resource.",
			},
			"ldap_hostname": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Hostname of the LDAP server. Used by the rdpldap resource.",
			},
			"ldap_port": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Port of the LDAP server. Used by the rdpldap resource.",
			},
			"ldap_encryption_method": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "LDAP encryption method (e.g. none, ssl, starttls). Used by the ldap and rdpldap resources.",
			},
			"ldap_search_bind_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DN used to bind for user searches. Used by the ldap and rdpldap resources.",
			},
			"ldap_search_bind_password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password for `ldap_search_bind_dn`. Used by the ldap and rdpldap resources.",
			},
			"ldap_user_name_attribute": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Attribute holding the username. Used by the ldap and rdpldap resources.",
			},
			"ldap_user_base_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Base DN user searches start from. Used by the ldap and rdpldap resources.",
			},
			"old_version": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Target a pre-2.x ProxySQL admin interface. Used by the proxysql resource.",
			},
			"proxysql_admin_port": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Port of the ProxySQL admin interface. Used by the proxysql resource.",
			},
			"proxysql_admin_username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for the ProxySQL admin interface. Used by the proxysql resource.",
			},
			"proxysql_admin_password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password for the ProxySQL admin interface. Used by the proxysql resource.",
			},
			"proxysql_hostgroup_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Hostgroup id queries are routed to. Used by the proxysql resource.",
			},
			"is_redis_labs": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "The target is a Redis Enterprise Cloud (Redis Labs) endpoint. Used by the redis resource.",
			},
			"machine": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Heroku API host. Used by the heroku resource.",
			},
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "API token. Used by the heroku resource.",
			},
			"resource": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Confluent resource (cluster) identifier. Used by the confluent resource.",
			},
			"service_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Oracle service name. Used by the oracle resource.",
			},
			"api_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "API secret paired with `api_key`. Used by the confluent resource.",
			},
			"redacted_digests": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "Opaque server fingerprints of the write-only secret arguments, used to detect " +
					"out-of-band secret changes on refresh. Not comparable across resources or workspaces.",
			},
			"targets": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of RDP targets. Used by the adaptive_rdp resource. Each block is one Windows host with its own credentials. Rejected on any other resource type.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Unique identifier for the target within the fleet.",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Human-friendly name shown in the in-browser target picker.",
						},
						"host": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Hostname or IP of the Windows server.",
						},
						"port": {
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     3389,
							Description: "RDP port. Defaults to 3389.",
						},
						"username": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Username to authenticate with the target.",
						},
						"password": {
							Type:        schema.TypeString,
							Required:    true,
							Sensitive:   true,
							Description: "Password to authenticate with the target.",
						},
						"domain": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Optional Windows domain for the target.",
						},
						"record": {
							Type:        schema.TypeBool,
							Optional:    true,
							Description: "Per-target session-recording override. If unset, inherits the global recording setting (COLLECT_RDP_RECORDINGS).",
						},
					},
				},
			},
		},
	}
}

// Returns a YAML marshallable struct for the integration configuration
func schemaToResourceIntegrationConfiguration(d *schema.ResourceData, intType string) (any, error) {
	// The provider shares one flat schema across all integration types, so a
	// `targets` block on any other type would be accepted and then silently
	// ignored. Reject it so the config can't drift from what actually runs.
	if _, ok := d.GetOk("targets"); ok && intType != "adaptive_rdp" {
		return nil, fmt.Errorf("`targets` is only supported by resources of type \"adaptive_rdp\", not %q", intType)
	}
	// Same reasoning for the AWS RDS IAM block: only postgres reads it, so
	// setting it on any other type would be silently dropped. The static key
	// pair is exempt for awselasticcache, which reads the same two arguments
	// for its own AWS access-control lookups.
	if intType != "postgres" {
		keys := []string{"use_rds_iam", "use_irsa", "aws_region", "aws_role_arn", "aws_service_account"}
		if intType != "awselasticcache" {
			keys = append(keys, "aws_access_key_id", "aws_secret_access_key")
		}
		for _, key := range keys {
			if _, ok := d.GetOk(key); ok {
				return nil, fmt.Errorf("`%s` is only supported by resources of type \"postgres\", not %q", key, intType)
			}
		}
	}
	switch intType {
	case "aws":
		return integrations.SchemaToAWSIntegrationConfiguration(d), nil
	case "awsredshift":
		return integrations.SchemaToAWSRedshiftIntegrationConfiguration(d), nil
	case "azure":
		return integrations.SchemaToAzureIntegrationConfiguration(d), nil
	case "azureactivedirectory":
		return integrations.SchemaToAzureActiveDirectoryIntegrationConfiguration(d), nil
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
		cfg := integrations.SchemaToRDPWindowsIntegrationConfiguration(d)
		if cfg.Password == "" {
			return nil, errors.New("rdp_windows requires a non-empty `password`; RDP connections with blank passwords fail to authenticate")
		}
		return cfg, nil
	case "adaptive_rdp":
		return integrations.SchemaToAdaptiveRDPIntegrationConfiguration(d)
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
	case "msteams_workflow":
		return integrations.SchemaToMSTeamsWorkflowIntegrationConfiguration(d), nil
	case "1password":
		return integrations.SchemaToOnePasswordIntegrationConfiguration(d), nil
	case "adaptiveencrypt":
		return integrations.SchemaToAdaptiveEncryptIntegrationConfiguration(d), nil
	case "adaptiveremotedesktop":
		return integrations.SchemaToAdaptiveRemoteDesktopIntegrationConfiguration(d), nil
	case "avastbusinessedr":
		return integrations.SchemaToAvastBusinessEDRIntegrationConfiguration(d), nil
	case "awscloudwatchlogs":
		return integrations.SchemaToAWSCloudWatchLogsIntegrationConfiguration(d), nil
	case "awselasticcache":
		return integrations.SchemaToAWSElastiCacheIntegrationConfiguration(d), nil
	case "awsdocumentdb_aws_secret_manager":
		return integrations.SchemaToAWSDocumentDBSecretsManagerIntegrationConfiguration(d), nil
	case "azure_documentdb":
		return integrations.SchemaToAzureDocumentDBIntegrationConfiguration(d), nil
	case "big_query":
		return integrations.SchemaToBigQueryIntegrationConfiguration(d), nil
	case "chrome":
		return integrations.SchemaToChromeIntegrationConfiguration(d), nil
	case "cockroachdb_aws_secrets_manager":
		return integrations.SchemaToCockroachDBSecretsManagerIntegrationConfiguration(d), nil
	case "confluent":
		return integrations.SchemaToConfluentIntegrationConfiguration(d), nil
	case "digitalocean":
		return integrations.SchemaToDigitalOceanIntegrationConfiguration(d), nil
	case "fortinet_analyzer":
		return integrations.SchemaToFortinetAnalyzerIntegrationConfiguration(d), nil
	case "fortinet_manager":
		return integrations.SchemaToFortinetManagerIntegrationConfiguration(d), nil
	case "heroku":
		return integrations.SchemaToHerokuIntegrationConfiguration(d), nil
	case "ivanti":
		return integrations.SchemaToIvantiIntegrationConfiguration(d), nil
	case "juniper_sw":
		return integrations.SchemaToJuniperSWIntegrationConfiguration(d), nil
	case "kafka":
		return integrations.SchemaToKafkaIntegrationConfiguration(d), nil
	case "ldap":
		return integrations.SchemaToLDAPIntegrationConfiguration(d), nil
	case "mongodb-do":
		return integrations.SchemaToMongoDBDOIntegrationConfiguration(d), nil
	case "mongodb36":
		return integrations.SchemaToMongoDB36IntegrationConfiguration(d), nil
	case "oracle":
		return integrations.SchemaToOracleIntegrationConfiguration(d), nil
	case "proxysql":
		return integrations.SchemaToProxySQLIntegrationConfiguration(d), nil
	case "rdpldap":
		return integrations.SchemaToRDPLdapIntegrationConfiguration(d), nil
	case "redis":
		return integrations.SchemaToRedisIntegrationConfiguration(d), nil
	case "securetunnels":
		return integrations.SchemaToSecureTunnelsIntegrationConfiguration(d), nil
	case "slack_webhook":
		return integrations.SchemaToSlackWebhookIntegrationConfiguration(d), nil
	case "sophos_fw":
		return integrations.SchemaToSophosFWIntegrationConfiguration(d), nil
	case "vnc":
		return integrations.SchemaToVNCIntegrationConfiguration(d), nil
	case "windows-server-groups":
		return integrations.SchemaToWindowsServerGroupsIntegrationConfiguration(d), nil
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

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := integrations.DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateResource(ctx, rName, integrations.WireType(iType), config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)

	// Best-effort: capture the server's secret fingerprints so the next refresh
	// can tell whether a credential changed out-of-band.
	if r, rerr := client.ReadResource(ctx, resp.ID); rerr == nil && r != nil {
		if err := d.Set("redacted_digests", r.RedactedDigests); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func ResourceAdaptiveResourceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	r, err := client.ReadResource(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out-of-band — drop it from state so Terraform recreates it.
	if r == nil {
		return notFoundGone(d)
	}

	// An import has no prior state to reconcile against, so everything comes
	// from the server. On refresh we keep what the configuration wrote and
	// overwrite only what the server actually reports, which is what preserves
	// the credentials it withholds.
	isImport := isImportRead(d, "name")
	if missingOnImport(d, isImport, r.Name) {
		return nil
	}

	if err := d.Set("name", r.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", integrations.ProviderType(r.IntegrationType)); err != nil {
		return diag.FromErr(err)
	}
	if err := setStringSet(d, "tags", r.UserTags); err != nil {
		return diag.FromErr(err)
	}
	if isImport || d.Get("default_cluster").(string) != "" {
		if err := d.Set("default_cluster", r.DefaultCluster); err != nil {
			return diag.FromErr(err)
		}
	}

	// Secret drift detection: the server withholds secret values but returns an
	// opaque fingerprint per redacted key. A fingerprint that differs from the
	// recorded one means the secret changed out-of-band — surface that by
	// blanking the corresponding argument, so the next plan shows the
	// configuration's value being re-applied. Keys with no recorded fingerprint
	// (import, first refresh after upgrade, older servers) are only recorded.
	cfg := r.Configuration
	if recorded := stateStringMap(d, "redacted_digests"); len(recorded) > 0 && len(r.RedactedDigests) > 0 {
		if changed := changedDigestKeys(recorded, r.RedactedDigests); len(changed) > 0 {
			cfg = make(map[string]interface{}, len(r.Configuration)+len(changed))
			for k, v := range r.Configuration {
				cfg[k] = v
			}
			for _, k := range changed {
				cfg[k] = ""
			}
		}
	}
	if err := integrations.ApplyIntegrationConfig(d, r.IntegrationType, cfg); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("redacted_digests", r.RedactedDigests); err != nil {
		return diag.FromErr(err)
	}

	if isImport && len(r.RedactedKeys) > 0 {
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("resource %q was imported without its credentials", r.Name),
			Detail: fmt.Sprintf("The server withholds %s. Set the matching arguments in your configuration, "+
				"or the next update will clear them.", strings.Join(r.RedactedKeys, ", ")),
		}}
	}
	return nil
}

// changedDigestKeys returns the keys whose fingerprint differs between the
// recorded and current digest maps. Keys present on only one side are not
// drift: they show up when a secret argument is added or removed by the
// configuration (the regular diff covers that), or when digests are recorded
// for the first time.
func changedDigestKeys(recorded, current map[string]string) []string {
	var changed []string
	for k, cur := range current {
		if prev, ok := recorded[k]; ok && prev != cur {
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return changed
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

	userTags, err := integrations.TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := integrations.DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, integrations.WireType(iType), config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	// Best-effort refresh of the secret fingerprints after the write.
	if r, rerr := client.ReadResource(ctx, resourceID); rerr == nil && r != nil {
		if err := d.Set("redacted_digests", r.RedactedDigests); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
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
