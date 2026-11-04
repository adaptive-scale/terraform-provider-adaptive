package integrations

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// ApplyIntegrationConfig is the inverse of the SchemaTo* builders: it folds the
// configuration the read endpoint returned back onto the resource attributes.
//
// Correctly redacted backend responses omit credential values; do not assume
// every backend format/classifier has complete coverage. A missing key leaves
// its attribute untouched. Present values are normally written, including an
// explicit empty value for a clear; mappings handle contract-specific nulls.
// Keys the forward mapping derives (usePassword, the
// hardcoded version/type constants) are skipped: they
// carry no user intent to reconcile.
func ApplyIntegrationConfig(d *schema.ResourceData, integrationType string, cfg map[string]interface{}) error {
	if len(cfg) == 0 {
		return nil
	}
	a := &configApplier{d: d, cfg: cfg}
	applyIntegrationMapping(a, integrationType)
	return a.err
}

// applyIntegrationMapping is the mapping itself, one arm per integration type.
// Split from ApplyIntegrationConfig so AttrForConfigKey can run the exact same
// switch in collect mode; the two can never disagree about which attribute a
// config key feeds.
func applyIntegrationMapping(a *configApplier, integrationType string) {
	switch ProviderType(integrationType) {
	case "aws":
		a.cloudString("aws_region_name", "region_name")
		a.cloudString("aws_access_key_id", "access_key_id")
		a.cloudString("aws_secret_access_key", "secret_access_key")
		a.boolean("use_role_arn", "use_role_arn")
		a.cloudString("aws_role_arn", "aws_role_arn")
		a.boolean("use_service_account", "use_service_account")
		a.cloudString("service_account", "service_account")
		a.boolean("create_if_not_exists", "create_if_not_exists")
		a.boolean("enable_allowed_commands", "enable_allowed_commands")
		a.cloudString("allowed_commands", "allowed_commands")
	case "azure":
		a.str("tenantID", "tenant_id")
		a.str("applicationID", "application_id")
		a.str("clientSecret", "client_secret")
	case "azureactivedirectory":
		a.str("domain", "domain")
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
		a.str("tenantID", "tenant_id")
		a.boolean("useTenant", "use_tenant")
	case "awsredshift":
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("sslMode", "ssl_mode")
	case "cockroachdb":
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("sslMode", "ssl_mode")
		a.str("rootCert", "tls_root_cert")
	case "gcp":
		a.str("project_id", "project_id")
		a.str("key_file", "key_file")
	case "google":
		a.cloudString("serviceAccount", "service_account")
		a.cloudString("adminEmail", "admin_email")

		a.str("domain", "domain")
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
	case "mongodb":
		a.cloudString("clientCertificate", "client_certificate")
		a.boolean("useTLS", "use_tls")
		a.cloudString("version", "version")

		a.str("uri", "uri")
	case "mysql":
		a.cloudString("clientCert", "client_cert")
		a.cloudString("clientKey", "client_key")
		a.cloudString("rootCert", "root_cert")
		a.boolean("oldVersion", "old_version")

		a.boolean("useRdsIam", "use_rds_iam")
		a.cloudString("authMode", "auth_mode")
		a.applyIAMIdentity()
		a.cloudString("sslMode", "ssl_mode")
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
	case "okta":
		a.str("domain", "domain")
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
	case "postgres":
		a.cloudString("authMode", "auth_mode")
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("sslMode", "ssl_mode")
		a.str("rootCert", "tls_root_cert")
		a.str("crtText", "tls_cert_file")
		a.str("keyText", "tls_key_file")
		a.boolean("useRdsIam", "use_rds_iam")
		a.applyIAMIdentity()
	case "ssh":
		// Authentication mode is derived from key; credentials are independent.
		a.str("username", "username")
		a.str("password", "password")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("sshKey", "key")
	case "kubernetes":
		a.str("apiserver", "api_server")
		a.str("token", "cluster_token")
		a.str("cacrt", "cluster_cert")
		a.str("namespace", "namespace")
		a.str("tolerationsBytes", "tolerations")
		a.str("annotationsBytes", "annotations")
		a.str("nodeSelectorBytes", "node_selector")
		a.str("affinityBytes", "node_affinity")
	case "awsdocumentdb":
		a.boolean("tlsEnabled", "tls_enabled")

		a.str("uri", "uri")
	case "zerotier":
		a.str("network_id", "network_id")
		a.str("api_token", "api_token")
	case "mongodb_atlas":
		a.str("uri", "uri")
		a.str("organization_id", "organization_id")
		a.str("public_key", "public_key")
		a.str("private_key", "private_key")
		a.str("project_id", "project_id")
	case "rdp_windows":
		a.boolean("allowFileTransfer", "allow_file_transfer")
		a.str("hostname", "hostname")
		a.str("password", "password")
		a.str("username", "username")
		a.str("port", "port")
	case "awssecretsmanager":
		a.str("aws_region_name", "aws_region_name")
		a.str("aws_arn", "aws_arn")
	case "postgres_aws_secrets_manager", "mysql_aws_secrets_manager",
		"sqlserver_aws_secrets_manager", "snowflake_aws_secrets_manager":
		a.str("arn", "arn")
		a.str("region", "region")
		a.str("secret_id", "secret_id")
	case "mongodb_aws_secrets_manager":
		a.str("arn", "arn")
		a.str("region", "region")
		a.str("secret_id", "secret_id")
		a.str("key", "key")
	case "sql_server":
		a.cloudString("sslMode", "ssl_mode")

		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("username", "username")
		a.str("password", "password")
	case "azuresqlserver":
		a.cloudString("sslMode", "ssl_mode")

		a.str("hostname", "hostname")
		a.str("port", "port")
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
	case "splunk":
		a.str("tokenID", "token_id")
		a.str("url", "url")
	case "datadog":
		a.str("dd_site", "dd_site")
		if a.collect != nil || a.cfg["dd_api_key"] != nil {
			a.str("dd_api_key", "dd_api_key")
		}
		if a.collect != nil || a.cfg["dd_app_key"] != nil {
			a.str("dd_app_key", "app_key")
		}
	case "coralogix":
		a.str("url", "uri")
		a.str("privateKey", "private_key")
		a.str("applicationName", "application_name")
		a.str("subSystemName", "sub_system_name")
	case "jumpcloud":
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
		a.str("domain", "domain")
		a.str("apiKey", "api_token")
	case "msteams":
		a.str("appID", "client_id")
		a.str("appKey", "client_secret")
		a.str("tenantID", "tenant_id")
	case "yugabytedb":
		a.str("hostname", "host")
		a.str("username", "username")
		a.str("password", "password")
		a.str("sslMode", "ssl_mode")
		a.str("rootCert", "root_cert")
		a.str("port", "port")
	case "onelogin":
		a.str("domain", "domain")
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
		a.str("apiClientID", "api_client_id")
		a.str("apiClientSecret", "api_client_secret")
	case "elasticsearch":
		a.str("url", "uri")
		a.str("username", "username")
		a.str("password", "password")
		a.str("index", "index")
	case "paloalto_ngfw":
		a.cloudString("password", "password")
		a.cloudString("username", "username")
		a.cloudString("hostname", "hostname")
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
		a.applyManagedBrowserSettings()
		a.cloudString("sshKey", "key")
		a.cloudString("port", "port")
	case "fortinet_ngfw", "cisco_ngfw", "hpe_switch":
		// `type` is a per-case constant of the forward mapping, not an input.
		a.cloudString("hostname", "hostname")
		a.cloudString("login_url", "uri")
		a.cloudString("port", "port")
		a.boolean("use_proxy", "use_proxy")
		a.cloudString("username", "username")
		a.cloudString("password", "password")
		a.cloudString("webui_port", "webui_port")
		a.applyManagedBrowserSettings()
		a.cloudString("sshKey", "key")
	case "snowflake":
		a.str("databaseAccount", "hostname")
		a.str("databaseUsername", "username")
		a.str("databasePassword", "password")
		a.str("databaseName", "database_name")
		a.str("warehouse", "warehouse")
		a.str("schema", "schema")
		a.str("clientcert", "clientcert")
		a.str("role", "role")
	case "custom_siem_webhook":
		a.str("url", "uri")
		a.str("sharedSecret", "shared_secret")
	case "aruba_sw":
		a.cloudString("hostname", "hostname")
		a.cloudString("username", "username")
		a.cloudString("password", "password")
		a.applyManagedBrowserSettings()
		a.cloudString("sshKey", "key")
		a.cloudString("port", "port")
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
	case "aruba_instant_on":
		a.arubaInstantOnAliases()
		a.cloudString("port", "port")
		a.cloudString("username", "username")
		// Null or missing credentials are withheld; explicit empty clears them.
		if a.collect != nil || a.cfg["password"] != nil {
			a.cloudString("password", "password")
		}
		if a.collect != nil || a.cfg["sshKey"] != nil {
			a.cloudString("sshKey", "key")
		}
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
		a.applyManagedBrowserSettings()
	case "syslog":
		a.boolean("tlsEnabled", "tls_enabled")
		a.cloudString("caCertificate", "tls_ca_cert")
		a.cloudString("clientCertificate", "client_certificate")
		a.cloudString("clientKey", "client_key")
		a.boolean("insecureSkipVerify", "tls_skip_verify")

		a.str("hostname", "hostname")
		a.str("port", "port")
		a.str("protocol", "protocol")
	case "customintegration":
		a.str("image", "image")
		a.str("service_account_name", "service_account_name")
	case "clickhouse":
		a.str("username", "username")
		a.str("password", "password")
		a.str("database", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.boolean("use-tls", "use_tls")
	case "keyspaces":
		a.cloudString("keyspace", "keyspace")
		a.cloudString("keyspaces_endpoint", "keyspaces_endpoint")
		a.cloudString("version", "version")
		a.cloudString("aws_region_name", "aws_region_name")
		a.cloudString("aws_access_key_id", "aws_access_key_id")
		a.cloudString("aws_secret_access_key", "aws_secret_access_key")
		a.boolean("use_role_arn", "use_role_arn")
		a.cloudString("aws_role_arn", "aws_role_arn")
		a.boolean("use_service_account", "use_service_account")
		a.cloudString("service_account", "service_account")
		a.boolean("create_if_not_exists", "create_if_not_exists")
	case "rabbitmq":
		a.str("url", "uri")
		a.str("username", "username")
		a.str("password", "password")
	case "azurecosmosnosql":
		a.str("endpoint", "uri")
		a.str("key", "api_token")
	case "services":
		a.boolean("enableTLS", "tls_enabled")

		a.str("urls", "urls")
	case "serverlist":
		a.hosts("hosts", "hosts")
		a.str("user", "default_user")
		a.str("sshKey", "key")
		a.str("password", "password")
	case "msteams_workflow":
		a.str("webhookURL", "webhook_url")
	case "1password":
		a.str("service_account", "service_account")
		a.boolean("use_connect_server", "use_connect_server")
		a.str("connect_server_url", "connect_server_url")
	case "adaptive_rdp":
		a.rdpTargets("targets", "targets")
	case "adaptiveencrypt":
		a.str("value", "value")
	case "adaptiveremotedesktop":
		a.str("cpu", "cpu")
		a.str("memory", "memory")
		a.str("storage", "storage")
	case "avastbusinessedr":
		a.str("clientID", "client_id")
		a.str("clientSecret", "client_secret")
	case "awscloudwatchlogs":
		a.str("aws_region", "aws_region_name")
		a.str("arn", "arn")
		a.str("log_group_name", "log_group_name")
		a.str("log_stream_name", "log_stream_name")
	case "awsdocumentdb_aws_secret_manager":
		a.str("arn", "arn")
		a.str("region", "region")
		a.str("secret_id", "secret_id")
		a.boolean("tlsEnabled", "tls_enabled")
	case "awselasticcache":
		a.boolean("useIamAuth", "use_iam_auth")
		a.boolean("useIrsa", "use_irsa")
		a.cloudString("aws_role_arn", "aws_role_arn")
		a.cloudString("aws_service_account", "aws_service_account")
		a.cloudString("cache_name", "cache_name")
		a.cloudString("cache_type", "cache_type")
		a.str("username", "username")
		a.str("password", "password")
		a.str("host", "host")
		a.str("port", "port")
		a.boolean("tlsEnabled", "tls_enabled")
		a.boolean("tlsSkipVerify", "tls_skip_verify")
		a.str("access_control_method", "access_control_method")
		a.str("access_control_group", "access_control_group")
		a.str("aws_region_name", "aws_region_name")
		a.str("aws_access_key_id", "aws_access_key_id")
		a.str("aws_secret_access_key", "aws_secret_access_key")
	case "azure_documentdb":
		a.str("uri", "uri")
	case "big_query":
		a.str("credential_json", "credential_json")
	case "chrome":
		a.str("url", "url")
		a.str("prestart", "prestart")
		a.str("automationMode", "automation_mode")
		a.str("fields", "fields")
		a.str("script", "script")
	case "cockroachdb_aws_secrets_manager":
		a.str("arn", "arn")
		a.str("region", "region")
		a.str("secret_id", "secret_id")
		a.str("db_user", "username")
		a.str("db_password", "password")
		a.str("db_name", "database_name")
		a.str("db_host", "host")
		a.str("db_port", "port")
		a.str("db_rootCert", "root_cert")
		a.str("sslMode", "ssl_mode")
	case "confluent":
		a.str("organization_id", "organization_id")
		a.str("username", "username")
		a.str("password", "password")
		a.str("resource", "resource")
		a.str("apiKey", "api_key")
		a.str("apiSecret", "api_secret")
	case "digitalocean":
		a.str("api_token", "api_token")
	case "fortinet_analyzer", "fortinet_manager":
		a.cloudString("username", "username")
		a.cloudString("password", "password")
		a.cloudString("hostname", "hostname")
		a.cloudString("port", "port")
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
		a.cloudString("sshKey", "key")
		a.applyManagedBrowserSettings()
	case "heroku":
		a.str("machine", "machine")
		a.str("username", "username")
		a.str("token", "token")
	case "ivanti":
		a.cloudString("username", "username")
		a.cloudString("password", "password")
		a.cloudString("hostname", "hostname")
		a.cloudString("port", "port")
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
		a.boolean("use_proxy", "use_proxy")
		a.cloudString("sshKey", "key")
		a.applyManagedBrowserSettings()
	case "juniper_sw", "sophos_fw":
		a.cloudString("username", "username")
		a.cloudString("password", "password")
		a.cloudString("hostname", "hostname")
		a.cloudString("port", "port")
		a.cloudString("sshKey", "key")
		a.applyManagedBrowserSettings()
		a.cloudString("webui_port", "webui_port")
		a.cloudString("login_url", "login_url")
	case "kafka":
		a.boolean("useMskIam", "use_msk_iam")
		a.cloudString("bootstrapServers", "bootstrap_servers")
		a.applyIAMIdentity()
		a.str("client_configuration", "client_configuration")
	case "ldap":
		a.str("hostname", "hostname")
		a.str("port", "port")
		a.str("ldap-encryption-method", "ldap_encryption_method")
		a.str("ldap-search-bind-dn", "ldap_search_bind_dn")
		a.str("ldap-search-bind-password", "ldap_search_bind_password")
		a.str("ldap-user-name-attribute", "ldap_user_name_attribute")
		a.str("ldap-user-base-dn", "ldap_user_base_dn")
	case "mongodb-do":
		a.str("dbname", "database_name")
		a.str("api_token", "api_token")
	case "mongodb36":
		a.str("uri", "uri")
		a.str("clientCertificate", "client_certificate")
		a.boolean("useTLS", "use_tls")
	case "oracle":
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("username", "username")
		a.str("password", "password")
		a.str("service_name", "service_name")
	case "proxysql":
		a.str("username", "username")
		a.str("password", "password")
		a.str("databaseName", "database_name")
		a.str("hostname", "host")
		a.str("port", "port")
		a.str("sslMode", "ssl_mode")
		a.str("rootCert", "root_cert")
		a.str("clientCert", "client_cert")
		a.str("clientKey", "client_key")
		a.boolean("oldVersion", "old_version")
		a.str("proxysqlAdminPort", "proxysql_admin_port")
		a.str("proxysqlAdminUsername", "proxysql_admin_username")
		a.str("proxysqlAdminPassword", "proxysql_admin_password")
		a.str("proxysqlHostgroupID", "proxysql_hostgroup_id")
	case "rdpldap":
		a.str("hostname", "hostname")
		a.str("port", "port")
		a.str("ldap-hostname", "ldap_hostname")
		a.str("ldap-port", "ldap_port")
		a.str("ldap-encryption-method", "ldap_encryption_method")
		a.str("ldap-search-bind-dn", "ldap_search_bind_dn")
		a.str("ldap-search-bind-password", "ldap_search_bind_password")
		a.str("ldap-user-base-dn", "ldap_user_base_dn")
		a.str("ldap-user-name-attribute", "ldap_user_name_attribute")
	case "redis":
		a.str("username", "username")
		a.str("password", "password")
		a.str("host", "host")
		a.str("port", "port")
		a.boolean("tlsEnabled", "tls_enabled")
		a.boolean("tlsSkipVerify", "tls_skip_verify")
		a.boolean("isRedisLabs", "is_redis_labs")
		a.str("crtText", "tls_cert_file")
		a.str("keyText", "tls_key_file")
		a.str("caText", "tls_ca_cert")
	case "securetunnels":
		// name only, and the server strips it from the configuration blob.
	case "slack_webhook":
		a.str("webhookURL", "webhook_url")
		a.str("apiToken", "api_token")
	case "vnc":
		a.hosts("hosts", "hosts")
		a.str("port", "port")
		a.str("password", "password")
	case "windows-server-groups":
		a.hosts("hosts", "hosts")
		a.str("username", "username")
		a.str("password", "password")
		a.str("port", "port")
	}
}

// configApplier writes configuration values onto resource attributes, keeping
// the first failure so a long mapping reads as a straight list of assignments.
//
// With collect non-nil it records the (config key -> attribute) pairs instead of
// writing anything. That is how AttrForConfigKey answers which attribute a
// server config key feeds without a second table: the mapping switch below is
// long enough (~82 integration types) that a parallel copy would rot the first
// time someone edited only one of the two.
type configApplier struct {
	d       *schema.ResourceData
	cfg     map[string]interface{}
	err     error
	collect map[string]string
}

func (a *configApplier) set(attr string, value interface{}) {
	if a.collect != nil {
		return
	}
	if err := a.d.Set(attr, value); err != nil && a.err == nil {
		a.err = fmt.Errorf("could not set %q: %w", attr, err)
	}
}

// record notes a mapping in collect mode. Called from the typed helpers, which
// are the only places that know both halves of the pair.
func (a *configApplier) record(cfgKey, attr string) bool {
	if a.collect == nil {
		return false
	}
	a.collect[cfgKey] = attr
	return true
}

// AttrForConfigKey resolves the schema attribute a server config key feeds, by
// running the real mapping switch in collect mode.
//
// Reports ok=false for a key the type does not map, and for the dotted or
// indexed paths the server produces for nested secrets ("targets[0].password") —
// those name no single attribute, so the caller reports them by name instead.
func AttrForConfigKey(integrationType, cfgKey string) (attr string, ok bool) {
	if cfgKey == "" {
		return "", false
	}
	a := &configApplier{
		// A non-empty cfg is required: ApplyIntegrationConfig returns early on
		// an empty one, and the helpers only fire for keys that are present.
		cfg:     map[string]interface{}{cfgKey: ""},
		collect: map[string]string{},
	}
	applyIntegrationMapping(a, integrationType)
	attr, ok = a.collect[cfgKey]
	return attr, ok
}

func (a *configApplier) str(cfgKey, attr string) {
	if a.record(cfgKey, attr) {
		return
	}
	if s, ok := GetStr(a.cfg, cfgKey); ok {
		a.set(attr, s)
	}
}

func (a *configApplier) boolean(cfgKey, attr string) {
	if a.record(cfgKey, attr) {
		return
	}
	if b, ok := GetBool(a.cfg, cfgKey); ok {
		a.set(attr, b)
	}
}

// hosts splits the newline-joined host list the server stores back into the
// list the configuration wrote. Ordering is preserved, so a list that did not
// drift compares equal.
func (a *configApplier) hosts(cfgKey, attr string) {
	if a.record(cfgKey, attr) {
		return
	}
	s, ok := GetStr(a.cfg, cfgKey)
	if !ok {
		return
	}
	var hosts []string
	for _, h := range strings.Split(s, "\n") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	a.set(attr, hosts)
}

// rdpTargets parses the marshalled target list adaptive_rdp stores as a single
// string back into the blocks the configuration declares.
func (a *configApplier) rdpTargets(cfgKey, attr string) {
	if a.record(cfgKey, attr) {
		return
	}
	s, ok := GetStr(a.cfg, cfgKey)
	if !ok {
		return
	}
	var targets []AdaptiveRDPTargetConfig
	if err := yaml.Unmarshal([]byte(s), &targets); err != nil {
		if a.err == nil {
			a.err = fmt.Errorf("could not parse %q: %w", cfgKey, err)
		}
		return
	}
	blocks := make([]interface{}, 0, len(targets))
	for _, t := range targets {
		block := map[string]interface{}{
			"id":       t.ID,
			"name":     t.Name,
			"host":     t.Host,
			"port":     t.Port,
			"username": t.Username,
			"password": t.Password,
			"domain":   t.Domain,
		}
		// record is tri-state: leave it out when the target inherits the
		// global recording setting.
		if t.Record != nil {
			block["record"] = *t.Record
		}
		blocks = append(blocks, block)
	}
	a.set(attr, blocks)
}

// GetStr reads a configuration value as a string. Values arrive as decoded JSON
// or YAML, so a port written unquoted in the stored configuration comes back as
// a number.
func GetStr(cfg map[string]interface{}, key string) (string, bool) {
	v, ok := cfg[key]
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case int:
		return strconv.Itoa(t), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case nil:
		return "", true
	}
	return "", false
}

// GetBool reads a configuration value as a bool, accepting the string spellings
// the resource forms submit for checkboxes.
func GetBool(cfg map[string]interface{}, key string) (bool, bool) {
	v, ok := cfg[key]
	if !ok {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		b, err := strconv.ParseBool(t)
		if err != nil {
			return false, false
		}
		return b, true
	}
	return false, false
}
