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
	return withCloudConfigOwnership(&schema.Resource{
		CreateContext: ResourceAdaptiveResourceCreate,
		ReadContext:   ResourceAdaptiveResourceRead,
		UpdateContext: ResourceAdaptiveResourceUpdate,
		DeleteContext: ResourceAdaptiveResourceDelete,
		CustomizeDiff: rdpTargetCustomizeDiff,
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
				Sensitive:   true,
				Description: "Root certificate for MySQL, ProxySQL, YugabyteDB, or CockroachDB Secrets Manager. Plain Postgres/CockroachDB use tls_root_cert.",
			},
			"ssl_mode": {
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					if i == "" {
						return nil, nil
					}
					validValues := []string{
						"prefer", "allow", "require", "verify-ca", "verify-full", "disable", "false", "skip-verify", "verify_identity", "verify_ca", "required", "preferred", "disabled",
					}

					if !slices.Contains(validValues, i.(string)) {
						return nil, []error{fmt.Errorf("invalid value for ssl_mode: %s", i)}
					}

					return nil, nil
				},
				Description: "SSL mode for MySQL, Postgres, CockroachDB, Redshift, ProxySQL, SQL Server, and Azure SQL Server. SQL Server/Azure SQL additionally support false and skip-verify; MySQL/ProxySQL accept only verify_identity, verify_ca, required, preferred, disabled, the legacy CLI prefix require (preserved literally), or empty (omit the CLI SSL-mode flag). PostgreSQL spellings such as verify-full are not MySQL/ProxySQL modes. Use required rather than legacy require for ProxySQL CLI export/apply compatibility. Absent MySQL mode defaults to require for compatibility; absent ProxySQL mode remains empty. ClickHouse uses use_tls.",
			},
			"admin_email":         {Type: schema.TypeString, Optional: true, Description: "Google directory delegated administrator email."},
			"allow_file_transfer": {Type: schema.TypeBool, Optional: true, Description: "Allow Windows RDP file transfer; explicit false disables it."},
			"auth_mode":           {Type: schema.TypeString, Optional: true, Description: "Legacy MySQL/Postgres authentication mode: password or aws_rds_iam. An explicitly configured use_rds_iam switch takes precedence; omit it to retain auth-mode-only configurations."},
			"use_msk_iam":         {Type: schema.TypeBool, Optional: true, Description: "Use AWS MSK IAM authentication for Kafka. Omitted switches retain backend nil/inference; explicit false disables IAM."},
			"use_iam_auth":        {Type: schema.TypeBool, Optional: true, Description: "Use AWS IAM authentication for ElastiCache. Omit to preserve observation; explicitly set false to disable."},
			"bootstrap_servers":   {Type: schema.TypeString, Optional: true, Description: "Comma-separated Kafka broker host:port string for MSK IAM. Empty falls back to bootstrap.servers in client_configuration."},
			"cache_name":          {Type: schema.TypeString, Optional: true, Description: "ElastiCache replication group or serverless cache identifier used for IAM token signing. Empty allows backend hostname inference."},
			"cache_type":          {Type: schema.TypeString, Optional: true, Description: "ElastiCache IAM signing resource type: serverless or replication_group. Empty allows backend hostname inference."},
			"use_rds_iam": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Authenticate Postgres/MySQL with short-lived AWS RDS IAM tokens. Enable IAM on RDS and provision the database user for the target engine. Omit to preserve backend auth-mode inference; explicit false overrides legacy auth_mode. TLS cannot be disabled.",
			},
			"use_irsa": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Select the AWS identity source for Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. False selects static keys. Omission preserves backend inference. Kafka sessions require a role or service account, not a bare platform identity.",
			},
			"aws_region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "AWS region for Postgres/MySQL RDS IAM or Kafka MSK IAM. Empty allows supported backend hostname/broker inference. ElastiCache and Keyspaces use aws_region_name.",
			},
			"aws_role_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Role ARN for AWS CLI, Keyspaces, Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. Omit to preserve observed ownership; explicit empty clears.",
			},
			"aws_access_key_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Static AWS access key ID for Keyspaces, ElastiCache, Postgres/MySQL RDS IAM, or Kafka MSK IAM. AWS CLI uses access_key_id instead.",
			},
			"aws_secret_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Static AWS secret access key for Keyspaces, ElastiCache, Postgres/MySQL RDS IAM, or Kafka MSK IAM. AWS CLI uses secret_access_key instead.",
			},
			"aws_service_account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes ServiceAccount identity for Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. Backend identity inference applies when use_irsa is omitted.",
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
				Sensitive:   true,
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
				Sensitive:   true,
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
				Sensitive:   true,
				Description: "Public key of the Atlas API key pair. Withheld by the server on read.",
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
				Description: "Keyspaces AWS region, or the AWS region of an AWS Secrets Manager secret. AWS CLI uses region_name instead.",
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
				Description: "Whether to use service account authentication. Used by AWS CLI and Keyspaces; omit to preserve observation and explicitly set false to disable.",
			},
			"use_role_arn": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Use AWS role authentication for AWS CLI or Keyspaces. Role authentication takes precedence when use_service_account is also true.",
			},
			"enable_allowed_commands": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "AWS CLI only: when enabled and both command and parameter are nonempty strings, the backend checks the requested service/subcommand against allowed_commands. If CLI parameters are absent or either value is missing, empty, or not a string, it falls back to an interactive /bin/bash shell before allowlist validation, even when enabled. This is not a command-only security boundary. Empty or whitespace-only allowed_commands means allow all, not deny all. This is separate from authorization resource scoping.",
			},
			"allowed_commands": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "AWS CLI only: comma-separated service:subcommand entries, used when enable_allowed_commands is true. Empty means allow all, NOT deny all.",
			},
			"keyspace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Keyspaces keyspace.",
			},
			"keyspaces_endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Keyspaces Cassandra endpoint hostname.",
			},
			"create_if_not_exists": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "AWS CLI/Keyspaces: create the Kubernetes service account if missing (NOT the keyspace). Omit to preserve observation; explicit false disables creation.",
			},
			"network_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The network ID for ZeroTier network",
			},
			"tls_root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "The root certificate to use for the Postgres-like resources.",
			},
			"tls_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
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
				Deprecated:  "Unsupported legacy argument with no wire mapping; use client_id for OAuth integrations.",
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The app ID",
			},
			"app_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "The application key. Used by Datadog (dd_app_key on the wire).",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Configuration version for Keyspaces or MongoDB. Other integration versions are provider-owned envelope constants.",
			},
			"database_account": {
				Deprecated:  "Unsupported legacy argument with no wire mapping; use hostname for Snowflake.",
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database account",
			},
			"database_username": {
				Deprecated:  "Unsupported legacy argument with no wire mapping; use username.",
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The database username",
			},
			"database_password": {
				Deprecated:  "Unsupported legacy argument with no wire mapping; use password.",
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
				Sensitive:   true,
				Description: "Google directory service-account JSON credential, 1Password service-account token, or AWS CLI/Keyspaces Kubernetes service-account name. Sensitive values remain in Terraform state. Directory backend redaction is not guaranteed for every representation.",
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
				Sensitive:   true,
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
				Description: "Connect over TLS. Used by Redis, ElastiCache, DocumentDB (including Secrets Manager), syslog, and services.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip verification of the server certificate. Used by Redis, ElastiCache, and syslog.",
			},
			"use_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Connect over TLS. Used by ClickHouse, MongoDB, and mongodb36. Omission preserves observed TLS on refresh/import; configure false explicitly to disable it. ClickHouse does not support ssl_mode.",
			},
			"tls_ca_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "CA certificate used to verify the server. Used by Redis and syslog.",
			},
			"client_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client certificate presented to the server. Used by MySQL and ProxySQL.",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client TLS private key. Used by MySQL, ProxySQL, and syslog (with client_certificate).",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client certificate bundle. Used by MongoDB, mongodb36, and syslog.",
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
				Description: "Browser automation mode for Chrome and the eleven supported managed network-device integrations.",
			},
			"fields": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Form fields to fill during automated login for Chrome and managed network-device browser sessions.",
			},
			"script": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Automation script to run for Chrome and managed network-device browser sessions.",
			},
			"prestart": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
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
				Description: "Use the old-version client behavior for MySQL or ProxySQL.",
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
			// Deliberately NOT refreshed by Read: these are the fingerprints as
			// of the last write, and they are the baseline the live ones are
			// compared against. Overwriting them on refresh (as the old
			// redacted_digests field did) destroys the evidence the comparison
			// just produced, which is why out-of-band secret changes never
			// reached the plan.
			"known_empty_secret_keys": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Server configuration keys of scalar sensitive strings explicitly observed empty. Retained until a verified provider write reconciles a subsequent credential addition; absent keys remain unknown.",
			},
			"applied_digests": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "Opaque server fingerprints of the write-only secret arguments as of the last " +
					"write by this provider, used to detect out-of-band secret changes on refresh. " +
					"Not comparable across resources or workspaces.",
			},
			"targets": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of RDP targets. Used by the adaptive_rdp resource. Each block is one Windows host with its own credentials. Rejected on any other resource type.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: integrations.ValidateAdaptiveRDPTargetString,
							Description:  "Unique identifier for the target within the fleet.",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Human-friendly name shown in the in-browser target picker.",
						},
						"host": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: integrations.ValidateAdaptiveRDPTargetString,
							Description:  "Hostname or IP of the Windows server.",
						},
						"port": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      3389,
							ValidateFunc: integrations.ValidateAdaptiveRDPTargetPort,
							Description:  "RDP port. Defaults to 3389.",
						},
						"username": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: integrations.ValidateAdaptiveRDPTargetString,
							Description:  "Username to authenticate with the target.",
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
	})
}

// Returns a YAML marshallable struct for the integration configuration
func schemaToResourceIntegrationConfiguration(d *schema.ResourceData, intType string) (any, error) {
	if err := validateResourceControlPresence(intType, d.GetRawConfig()); err != nil {
		return nil, err
	}
	if err := validateResourceContractInputs(intType, d.Get); err != nil {
		return nil, err
	}
	// The provider shares one flat schema across all integration types, so a
	// `targets` block on any other type would be accepted and then silently
	// ignored. Reject it so the config can't drift from what actually runs.
	if _, ok := d.GetOk("targets"); ok && intType != "adaptive_rdp" {
		return nil, fmt.Errorf("`targets` is only supported by resources of type \"adaptive_rdp\", not %q", intType)
	}
	// Cloud omission ownership requires the raw Terraform configuration before
	// this shared capability guard can permit a full-replacement update.
	if err := validateCloudConfigOwnership(intType, d.Id() != "", d.GetRawConfig()); err != nil {
		return nil, err
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
		if err := captureResourceSecretDigests(d, r); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func ResourceAdaptiveResourceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
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

	// Decode nested configuration against a detached state first. A malformed
	// targets document must not leave name, tags, or credentials half-refreshed.
	staged := ResourceAdaptiveResource().Data(d.State())
	if err := integrations.ApplyIntegrationConfig(staged, r.IntegrationType, r.Configuration); err != nil {
		return diag.FromErr(err)
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

	if err := integrations.ApplyIntegrationConfig(d, r.IntegrationType, r.Configuration); err != nil {
		return diag.FromErr(err)
	}

	// Secret drift: the server withholds secret values but returns an opaque
	// fingerprint per redacted key. A fingerprint that differs from the one
	// recorded at our last write means the value changed out-of-band.
	//
	// The signal is the fingerprint written onto the argument it belongs to.
	// Marking the argument rather than blanking it is what makes the drift reach
	// the plan at all: an argument absent from the configuration reads back as
	// "" either way, so blanking produced no diff and a secret the configuration
	// does not manage drifted silently. The argument is Sensitive, so the marker
	// renders as (sensitive value) and no value is disclosed — the attribute name
	// carries the information.
	//
	// A configured legacy argument can propose restoration of its desired
	// value. Omitted held credentials cannot safely be merged into a full
	// replacement: Update rejects them rather than silently clearing them.
	// Terraform may retain refreshed markers under ignore_changes, so Update
	// validates planned/configured values and the actual serialized payload
	// before sending any mutation; it does not assume Get reads raw input.
	//
	// An empty legacy baseline is unknown and may be seeded on refresh.
	// Per-key known-empty provenance is different: the first server fingerprint
	// for such a key is actionable drift, and refresh must not seed it away.
	// Two different reasons a drifted key cannot be marked, kept apart because
	// they mean different things to whoever reads the warning.
	// An explicit empty value is authoritative evidence of a clear. Missing
	// fields or digests are not: older servers and redacted reads omit them.
	// Materialize an empty computed map too: an empty-credential import must
	// not schedule a write just to change null into a known empty baseline.
	drifted, evidenceErr := refreshResourceSecretEvidence(d, r)
	if evidenceErr != nil {
		return diag.FromErr(evidenceErr)
	}
	var unmapped []string  // names no single attribute at all
	var notSecret []string // names one, but the schema does not call it sensitive
	if len(drifted) > 0 {
		for _, k := range drifted {
			attr, ok := integrations.AttrForConfigKey(r.IntegrationType, k)
			switch {
			case integrations.ProviderType(r.IntegrationType) == "adaptive_rdp" && k == "targets":
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Warning,
					Summary:  fmt.Sprintf("resource %q: RDP configuration changed outside this configuration", r.Name),
					Detail:   "The aggregate RDP configuration no longer matches the last-applied fingerprint. This warning does not attribute the change to any target or credential and does not force a Terraform update. Explicitly resupply the intended targets configuration to reconcile it.",
				})
			case !ok:
				unmapped = append(unmapped, k)
			case !isSecretStringAttr(attr):
				// Writing a fingerprint into an attribute the schema shows in
				// clear would render as the value turning into hex. The server
				// withholding a key this provider does not treat as sensitive is
				// a provider bug, so say so rather than blaming the key's shape.
				notSecret = append(notSecret, fmt.Sprintf("%s (attribute %q)", k, attr))
			default:
				if err := d.Set(attr, r.RedactedDigests[k]); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	if len(unmapped) > 0 {
		// Dotted and indexed paths, or keys outside the reverse mapping,
		// cannot be represented by a single scalar credential argument.
		// Reporting them by name keeps the drift visible rather than dropping it.
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("resource %q: a secret changed outside this configuration", r.Name),
			Detail: fmt.Sprintf("The value behind %s no longer matches what this provider last wrote, "+
				"but it maps to no single attribute and so cannot be shown as a plan diff.",
				strings.Join(unmapped, ", ")),
		})
	}
	if len(notSecret) > 0 {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("resource %q: a secret changed outside this configuration", r.Name),
			Detail: fmt.Sprintf("The value behind %s no longer matches what this provider last wrote, "+
				"but that attribute is not declared sensitive, so the change cannot be shown "+
				"without printing a fingerprint in clear. This is a provider bug — please report it.",
				strings.Join(notSecret, ", ")),
		})
	}

	if isImport && len(r.RedactedKeys) > 0 {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("resource %q was imported without its credentials", r.Name),
			Detail: fmt.Sprintf("The server withholds %s. Supply the matching arguments before updating this resource; "+
				"updates that would omit known server-held credentials are rejected. Set a supported optional string argument explicitly to \"\" only to authorize clearing it.", strings.Join(r.RedactedKeys, ", ")),
		})
	}
	return diags
}

// captureResourceSecretDigests records an acknowledged write (or initial read).
// Incomplete post-write reads retain the previous evidence, except for fields
// explicitly confirmed clear. Capture remains best-effort until the backend
// returns fingerprints atomically with the write response.
func captureResourceSecretDigests(d *schema.ResourceData, r *adaptive.ResourceReadResponse) error {
	// Validate the read before advancing either piece of credential evidence.
	staged := ResourceAdaptiveResource().Data(d.State())
	if err := integrations.ApplyIntegrationConfig(staged, r.IntegrationType, r.Configuration); err != nil {
		return err
	}
	applied := pruneClearedSecretDigests(stateStringMap(d, "applied_digests"), r.Configuration)
	known := knownEmptySecretKeys(d)
	for key, digest := range r.RedactedDigests {
		if digest != "" {
			applied[key] = digest
			delete(known, key)
		}
	}
	observeEmptySecretKeys(known, r)
	if err := d.Set("applied_digests", applied); err != nil {
		return err
	}
	return setKnownEmptySecretKeys(d, known)
}

// pruneClearedSecretDigests keeps last-write evidence unless the server
// explicitly reports the corresponding value as an empty string. Never infer
// a clear from an absent key, an omitted digest, or a failed read.
func pruneClearedSecretDigests(applied map[string]string, cfg map[string]interface{}) map[string]string {
	out := make(map[string]string, len(applied))
	for key, digest := range applied {
		if value, ok := cfg[key].(string); ok && value == "" {
			continue
		}
		out[key] = digest
	}
	return out
}

// isSecretStringAttr reports whether an attribute can carry a drift marker: a
// sensitive string. A non-string attribute (the adaptive_rdp targets list) has
// nowhere to put a fingerprint, and marking a non-sensitive one would print it.
func isSecretStringAttr(attr string) bool {
	sch, ok := ResourceAdaptiveResource().Schema[attr]
	return ok && sch.Type == schema.TypeString && sch.Sensitive
}

// driftedDigestKeys returns the keys whose fingerprint no longer matches the one
// recorded at the last write — either because the value behind it changed, or
// because the server now holds a secret where it recorded none.
//
// New keys are drift against a nonempty applied baseline. Read also checks
// known-empty per-key provenance, distinguishing a first addition from an
// unknown empty legacy baseline without inventing a server fingerprint.
//
// Keys that disappeared are not reported: the secret is gone, there is nothing
// to reconcile, and the ordinary diff already covers a configuration removing
// one.
func driftedDigestKeys(applied, current map[string]string) []string {
	var drifted []string
	for k, cur := range current {
		if prev, ok := applied[k]; !ok || prev != cur {
			drifted = append(drifted, k)
		}
	}
	sort.Strings(drifted)
	return drifted
}

func ResourceAdaptiveResourceUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	oldType, newType := d.GetChange("type")
	if err := validateResourceTypeTransition(d.Id(), oldType, newType); err != nil {
		return diag.FromErr(err)
	}
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

	if err := validateResourceSecretUpdate(ctx, d, client, config); err != nil {
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, integrations.WireType(iType), config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	// Best-effort refresh of the secret fingerprints after the write.
	if r, rerr := client.ReadResource(ctx, resourceID); rerr == nil && r != nil {
		if err := captureResourceSecretDigests(d, r); err != nil {
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
