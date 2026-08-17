package integrations

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// integrationTypes mirrors the resource `type` values the provider accepts. It
// is declared here rather than imported so the package tests standalone; a type
// added to the resource dispatch without a reverse mapping fails the round-trip
// below.
var integrationTypes = []string{
	"1password", "adaptive_rdp", "adaptiveencrypt", "adaptiveremotedesktop",
	"aruba_instant_on", "aruba_sw", "avastbusinessedr", "aws", "awscloudwatchlogs",
	"awsdocumentdb", "awsdocumentdb_aws_secret_manager", "awselasticcache",
	"awsredshift", "awssecretsmanager", "azure", "azure_documentdb",
	"azureactivedirectory", "azurecosmosnosql", "azuresqlserver", "big_query",
	"chrome", "cisco_ngfw", "clickhouse", "cockroachdb",
	"cockroachdb_aws_secrets_manager", "confluent", "coralogix",
	"custom_siem_webhook", "customintegration", "datadog", "digitalocean",
	"elasticsearch", "fortinet_analyzer", "fortinet_manager", "fortinet_ngfw",
	"gcp", "google", "heroku", "hpe_switch", "ivanti", "jumpcloud", "juniper_sw",
	"kafka", "keyspaces", "kubernetes", "ldap", "mongodb", "mongodb-do",
	"mongodb36", "mongodb_atlas", "mongodb_aws_secrets_manager", "msteams",
	"msteams_workflow", "mysql", "mysql_aws_secrets_manager", "okta", "onelogin",
	"oracle", "paloalto_ngfw", "postgres", "postgres_aws_secrets_manager",
	"proxysql", "rabbitmq", "rdp_windows", "rdpldap", "redis", "securetunnels",
	"serverlist", "services", "slack_webhook", "snowflake",
	"snowflake_aws_secrets_manager", "sophos_fw", "splunk", "sql_server",
	"sqlserver_aws_secrets_manager", "ssh", "syslog", "vnc",
	"windows-server-groups", "yugabytedb", "zerotier",
}

// buildIntegrationConfig dispatches a type onto its SchemaTo* builder, mirroring
// the dispatch the adaptive_resource implementation performs.
func buildIntegrationConfig(typ string, d *schema.ResourceData) (interface{}, error) {
	switch typ {
	case "1password":
		return SchemaToOnePasswordIntegrationConfiguration(d), nil
	case "adaptive_rdp":
		return SchemaToAdaptiveRDPIntegrationConfiguration(d)
	case "adaptiveencrypt":
		return SchemaToAdaptiveEncryptIntegrationConfiguration(d), nil
	case "adaptiveremotedesktop":
		return SchemaToAdaptiveRemoteDesktopIntegrationConfiguration(d), nil
	case "aruba_instant_on":
		return SchemaToArubaInstantOnIntegrationConfiguration(d), nil
	case "aruba_sw":
		return SchemaToArubaSWIntegrationConfiguration(d), nil
	case "avastbusinessedr":
		return SchemaToAvastBusinessEDRIntegrationConfiguration(d), nil
	case "aws":
		return SchemaToAWSIntegrationConfiguration(d), nil
	case "awscloudwatchlogs":
		return SchemaToAWSCloudWatchLogsIntegrationConfiguration(d), nil
	case "awsdocumentdb":
		return SchemaToAWSDocumentDBIntegrationConfiguration(d), nil
	case "awsdocumentdb_aws_secret_manager":
		return SchemaToAWSDocumentDBSecretsManagerIntegrationConfiguration(d), nil
	case "awselasticcache":
		return SchemaToAWSElastiCacheIntegrationConfiguration(d), nil
	case "awsredshift":
		return SchemaToAWSRedshiftIntegrationConfiguration(d), nil
	case "awssecretsmanager":
		return SchemaToAWSSecretsManagerConfiguration(d), nil
	case "azure":
		return SchemaToAzureIntegrationConfiguration(d), nil
	case "azure_documentdb":
		return SchemaToAzureDocumentDBIntegrationConfiguration(d), nil
	case "azureactivedirectory":
		return SchemaToAzureActiveDirectoryIntegrationConfiguration(d), nil
	case "azurecosmosnosql":
		return SchemaToAzureCosmosNoSQLIntegrationConfiguration(d), nil
	case "azuresqlserver":
		return SchemaToAzureSQLServerIntegrationConfiguration(d)
	case "big_query":
		return SchemaToBigQueryIntegrationConfiguration(d), nil
	case "chrome":
		return SchemaToChromeIntegrationConfiguration(d), nil
	case "cisco_ngfw":
		return SchemaToCiscoNGFWIntegrationConfiguration(d), nil
	case "clickhouse":
		return SchemaToClickHouseIntegrationConfiguration(d), nil
	case "cockroachdb":
		return SchemaToCockroachDBIntegrationConfiguration(d), nil
	case "cockroachdb_aws_secrets_manager":
		return SchemaToCockroachDBSecretsManagerIntegrationConfiguration(d), nil
	case "confluent":
		return SchemaToConfluentIntegrationConfiguration(d), nil
	case "coralogix":
		return SchemaToCoralogixIntegrationConfiguration(d), nil
	case "custom_siem_webhook":
		return SchemaToCustomSIEMWebhookIntegrationConfiguration(d), nil
	case "customintegration":
		return SchemaToCustomIntegrationConfiguration(d), nil
	case "datadog":
		return SchemaToDatadogIntegrationConfiguration(d), nil
	case "digitalocean":
		return SchemaToDigitalOceanIntegrationConfiguration(d), nil
	case "elasticsearch":
		return SchemaToElasticsearchIntegrationConfiguration(d), nil
	case "fortinet_analyzer":
		return SchemaToFortinetAnalyzerIntegrationConfiguration(d), nil
	case "fortinet_manager":
		return SchemaToFortinetManagerIntegrationConfiguration(d), nil
	case "fortinet_ngfw":
		return SchemaToFortinetNGFWIntegrationConfiguration(d), nil
	case "gcp":
		return SchemaToGCPIntegrationConfiguration(d), nil
	case "google":
		return SchemaToGoogleOAuthIntegrationConfiguration(d), nil
	case "heroku":
		return SchemaToHerokuIntegrationConfiguration(d), nil
	case "hpe_switch":
		return SchemaToHPESwitchIntegrationConfiguration(d), nil
	case "ivanti":
		return SchemaToIvantiIntegrationConfiguration(d), nil
	case "jumpcloud":
		return SchemaToJumpCloudIntegrationConfiguration(d), nil
	case "juniper_sw":
		return SchemaToJuniperSWIntegrationConfiguration(d), nil
	case "kafka":
		return SchemaToKafkaIntegrationConfiguration(d), nil
	case "keyspaces":
		return SchemaToKeyspacesIntegrationConfiguration(d), nil
	case "kubernetes":
		return SchemaToKubernetesIntegrationConfiguration(d), nil
	case "ldap":
		return SchemaToLDAPIntegrationConfiguration(d), nil
	case "mongodb":
		return SchemaToMongoIntegrationConfiguration(d), nil
	case "mongodb-do":
		return SchemaToMongoDBDOIntegrationConfiguration(d), nil
	case "mongodb36":
		return SchemaToMongoDB36IntegrationConfiguration(d), nil
	case "mongodb_atlas":
		return SchemaToMongoAtlasIntegrationConfiguration(d), nil
	case "mongodb_aws_secrets_manager":
		return SchemaToMongoAWSIntegrationConfiguration(d), nil
	case "msteams":
		return SchemaToMSTeamsIntegrationConfiguration(d), nil
	case "msteams_workflow":
		return SchemaToMSTeamsWorkflowIntegrationConfiguration(d), nil
	case "mysql":
		return SchemaToMySQLIntegrationConfiguration(d), nil
	case "mysql_aws_secrets_manager":
		return SchemaToMySQLAWSIntegrationConfiguration(d), nil
	case "okta":
		return SchemaToOktaIntegrationConfiguration(d), nil
	case "onelogin":
		return SchemaToOneLoginIntegrationConfiguration(d), nil
	case "oracle":
		return SchemaToOracleIntegrationConfiguration(d), nil
	case "paloalto_ngfw":
		return SchemaToPaloAltoNGFWIntegrationConfiguration(d), nil
	case "postgres":
		return SchemaToPostgresIntegrationConfiguration(d), nil
	case "postgres_aws_secrets_manager":
		return SchemaToPostgresAWSIntegrationConfiguration(d), nil
	case "proxysql":
		return SchemaToProxySQLIntegrationConfiguration(d), nil
	case "rabbitmq":
		return SchemaToRabbitMQIntegrationConfiguration(d), nil
	case "rdp_windows":
		return SchemaToRDPWindowsIntegrationConfiguration(d), nil
	case "rdpldap":
		return SchemaToRDPLdapIntegrationConfiguration(d), nil
	case "redis":
		return SchemaToRedisIntegrationConfiguration(d), nil
	case "securetunnels":
		return SchemaToSecureTunnelsIntegrationConfiguration(d), nil
	case "serverlist":
		return SchemaToServerListIntegrationConfiguration(d)
	case "services":
		return SchemaToServiceListIntegrationConfiguration(d), nil
	case "slack_webhook":
		return SchemaToSlackWebhookIntegrationConfiguration(d), nil
	case "snowflake":
		return SchemaToSnowflakeIntegrationConfiguration(d), nil
	case "snowflake_aws_secrets_manager":
		return SchemaToSnowflakeAWSIntegrationConfiguration(d), nil
	case "sophos_fw":
		return SchemaToSophosFWIntegrationConfiguration(d), nil
	case "splunk":
		return SchemaToSplunkIntegrationConfiguration(d), nil
	case "sql_server":
		return SchemaToSQLServerIntegrationConfiguration(d), nil
	case "sqlserver_aws_secrets_manager":
		return SchemaToSQLServerAWSIntegrationConfiguration(d), nil
	case "ssh":
		return SchemaToSSHIntegrationConfiguration(d), nil
	case "syslog":
		return SchemaToSyslogIntegrationConfiguration(d), nil
	case "vnc":
		return SchemaToVNCIntegrationConfiguration(d), nil
	case "windows-server-groups":
		return SchemaToWindowsServerGroupsIntegrationConfiguration(d), nil
	case "yugabytedb":
		return SchemaToYugabyteDBIntegrationConfiguration(d), nil
	case "zerotier":
		return SchemaToZeroTierIntegrationConfiguration(d), nil
	}
	return nil, nil
}

// roundTripStringAttrs is every string attribute the builders read.
var roundTripStringAttrs = []string{
	"access_control_group", "access_control_method", "access_key_id",
	"annotations", "api_client_id", "api_client_secret", "api_key", "api_server",
	"api_secret", "api_token", "application_id", "application_name", "arn",
	"automation_mode", "aws_access_key_id", "aws_arn", "aws_region",
	"aws_region_name", "aws_role_arn", "aws_secret_access_key",
	"aws_service_account", "client_cert", "client_certificate",
	"client_configuration", "client_id", "client_key", "client_secret",
	"clientcert", "cluster_cert", "cluster_token", "connect_server_url", "cpu",
	"credential_json", "database_name", "dd_api_key", "dd_site", "default_user",
	"domain", "fields", "host", "hostname", "image", "index", "key", "key_file",
	"ldap_encryption_method", "ldap_hostname", "ldap_port",
	"ldap_search_bind_dn", "ldap_search_bind_password", "ldap_user_base_dn",
	"ldap_user_name_attribute", "log_group_name", "log_stream_name", "login_url",
	"machine", "memory", "name", "namespace", "network_id", "node_affinity",
	"node_selector", "organization_id", "password", "port", "prestart",
	"private_key", "project_id", "protocol", "proxysql_admin_password",
	"proxysql_admin_port", "proxysql_admin_username", "proxysql_hostgroup_id",
	"public_key", "region", "region_name", "resource", "role", "root_cert",
	"schema", "script", "secret_access_key", "secret_id", "service_account",
	"service_account_name", "service_name", "shared_secret", "ssl_mode",
	"storage", "sub_system_name", "tenant_id", "tls_ca_cert", "tls_cert_file",
	"tls_key_file", "tls_root_cert", "token", "token_id", "tolerations", "uri",
	"url", "urls", "username", "value", "warehouse", "webhook_url", "webui_port",
}

// roundTripBoolAttrs is every bool attribute the builders read.
var roundTripBoolAttrs = []string{
	"create_if_not_exists", "is_redis_labs", "old_version", "tls_enabled",
	"tls_skip_verify", "use_connect_server", "use_irsa", "use_proxy",
	"use_rds_iam", "use_service_account", "use_tenant", "use_tls",
}

func roundTripSchema() map[string]*schema.Schema {
	s := map[string]*schema.Schema{
		"hosts": {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}},
		"targets": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id":       {Type: schema.TypeString, Required: true},
					"name":     {Type: schema.TypeString, Optional: true},
					"host":     {Type: schema.TypeString, Required: true},
					"port":     {Type: schema.TypeInt, Optional: true, Default: 3389},
					"username": {Type: schema.TypeString, Required: true},
					"password": {Type: schema.TypeString, Required: true, Sensitive: true},
					"domain":   {Type: schema.TypeString, Optional: true},
					"record":   {Type: schema.TypeBool, Optional: true},
				},
			},
		},
	}
	for _, k := range roundTripStringAttrs {
		s[k] = &schema.Schema{Type: schema.TypeString, Optional: true}
	}
	for _, k := range roundTripBoolAttrs {
		s[k] = &schema.Schema{Type: schema.TypeBool, Optional: true}
	}
	return s
}

// roundTripFixture gives every attribute a distinctive non-zero value, so a
// field the reverse mapping forgets shows up as a difference rather than as two
// matching zero values.
func roundTripFixture() map[string]interface{} {
	raw := map[string]interface{}{
		"hosts": []interface{}{"h1", "h2"},
		"targets": []interface{}{
			map[string]interface{}{
				"id": "target-1", "name": "t1", "host": "10.0.0.5", "port": 3390,
				"username": "admin", "password": "pw", "domain": "corp",
			},
		},
	}
	for _, k := range roundTripStringAttrs {
		raw[k] = k + "-val"
	}
	for _, k := range roundTripBoolAttrs {
		raw[k] = true
	}
	return raw
}

// configToMap marshals a config struct to YAML and back into the string-keyed
// map shape the read endpoint returns.
func configToMap(t *testing.T, cfg interface{}) map[string]interface{} {
	t.Helper()
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[interface{}]interface{}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := make(map[string]interface{}, len(raw))
	for k, v := range raw {
		ks, ok := k.(string)
		if !ok {
			t.Fatalf("non-string yaml key %v", k)
		}
		out[ks] = v
	}
	return out
}

// TestIntegrationConfigRoundTrip is the completeness guard: for every supported
// integration type, build a config from a fully-populated resource, feed it
// through ApplyIntegrationConfig onto a fresh resource, rebuild, and require the
// two configs to be identical. A type added to the forward dispatch but
// forgotten in ApplyIntegrationConfig fails here.
func TestIntegrationConfigRoundTrip(t *testing.T) {
	for _, typ := range integrationTypes {
		t.Run(typ, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, roundTripSchema(), roundTripFixture())
			cfg1, err := buildIntegrationConfig(typ, d)
			if err != nil {
				t.Fatalf("forward build: %v", err)
			}
			if cfg1 == nil {
				t.Fatalf("no forward builder for %q", typ)
			}
			m := configToMap(t, cfg1)
			// The server strips the name key from read responses.
			delete(m, "name")

			back := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{
				"name": "name-val",
			})
			if err := ApplyIntegrationConfig(back, WireType(typ), m); err != nil {
				t.Fatalf("apply: %v", err)
			}

			cfg2, err := buildIntegrationConfig(typ, back)
			if err != nil {
				t.Fatalf("rebuild: %v", err)
			}
			m1 := configToMap(t, cfg1)
			m2 := configToMap(t, cfg2)
			if !reflect.DeepEqual(m1, m2) {
				t.Errorf("round-trip mismatch:\n forward: %v\n rebuilt: %v", m1, m2)
			}

			// Behavioral completeness: the reverse mapping must have populated
			// something beyond the envelope. securetunnels carries no fields
			// past its name, so it is exempt.
			if typ == "securetunnels" {
				return
			}
			bare := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{
				"name": "name-val",
			})
			if cfgBare, err := buildIntegrationConfig(typ, bare); err == nil {
				if reflect.DeepEqual(m2, configToMap(t, cfgBare)) {
					t.Errorf("ApplyIntegrationConfig set no fields for %q", typ)
				}
			}
		})
	}
}

// TestApplyIntegrationConfigRedaction pins the refresh semantics around
// server-side secret stripping: absent keys never clear prior values, present
// keys update them, and an empty value clears the attribute.
func TestApplyIntegrationConfigRedaction(t *testing.T) {
	d := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{
		"name":         "db",
		"host":         "h",
		"port":         "5432",
		"username":     "app",
		"password":     "s3cret",
		"tls_key_file": "KEY",
		"ssl_mode":     "require",
	})

	// Server response: secrets (password, keyText) stripped, hostname drifted,
	// databaseName cleared out-of-band.
	err := ApplyIntegrationConfig(d, "postgres", map[string]interface{}{
		"hostname":     "newhost",
		"port":         "5432",
		"username":     "app",
		"databaseName": "",
		"sslMode":      "require",
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := d.Get("password").(string); got != "s3cret" {
		t.Errorf("redacted password clobbered: %q", got)
	}
	if got := d.Get("tls_key_file").(string); got != "KEY" {
		t.Errorf("redacted key clobbered: %q", got)
	}
	if got := d.Get("host").(string); got != "newhost" {
		t.Errorf("hostname drift missed: %q", got)
	}
	if got := d.Get("database_name").(string); got != "" {
		t.Errorf("empty server value should clear the attribute, got %q", got)
	}
}

// TestApplyIntegrationConfigCoerces pins that a port stored unquoted, and a
// checkbox submitted as a string, still land on their string/bool attributes.
func TestApplyIntegrationConfigCoerces(t *testing.T) {
	d := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{"name": "n"})
	if err := ApplyIntegrationConfig(d, "redis", map[string]interface{}{
		"port":       5432,
		"tlsEnabled": "true",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := d.Get("port").(string); got != "5432" {
		t.Errorf("numeric port not coerced: %q", got)
	}
	if !d.Get("tls_enabled").(bool) {
		t.Error("string checkbox value not coerced")
	}
}

// TestApplyIntegrationConfigUsesWireType checks the reverse mapping resolves the
// stored wire spellings, including the legacy zerotier row.
func TestApplyIntegrationConfigUsesWireType(t *testing.T) {
	for _, wire := range []string{"zero_tier", "zerotier"} {
		d := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{"name": "n"})
		if err := ApplyIntegrationConfig(d, wire, map[string]interface{}{"network_id": "net-1"}); err != nil {
			t.Fatalf("apply %s: %v", wire, err)
		}
		if got := d.Get("network_id").(string); got != "net-1" {
			t.Errorf("apply %s: network_id = %q", wire, got)
		}
	}

	d := schema.TestResourceDataRaw(t, roundTripSchema(), map[string]interface{}{"name": "n"})
	if err := ApplyIntegrationConfig(d, "servicelist", map[string]interface{}{"urls": "a,b"}); err != nil {
		t.Fatalf("apply servicelist: %v", err)
	}
	if got := d.Get("urls").(string); got != "a,b" {
		t.Errorf("apply servicelist: urls = %q", got)
	}
}

// TestWireTypeMapping pins the two wire-name rewrites and their inverses.
func TestWireTypeMapping(t *testing.T) {
	cases := map[string]string{"services": "servicelist", "zerotier": "zero_tier", "postgres": "postgres"}
	for prog, wire := range cases {
		if got := WireType(prog); got != wire {
			t.Errorf("WireType(%q) = %q, want %q", prog, got, wire)
		}
		if got := ProviderType(wire); got != prog {
			t.Errorf("ProviderType(%q) = %q, want %q", wire, got, prog)
		}
	}
	// The legacy stored spelling still resolves to the program type.
	if got := ProviderType("zerotier"); got != "zerotier" {
		t.Errorf("ProviderType(zerotier legacy) = %q", got)
	}
}
