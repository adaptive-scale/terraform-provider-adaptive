package components

// configOwnershipTypes bounds omission preservation to actual backend contracts.
// These fields remain Optional, never globally Computed. Remote values remain
// readable; omission preserves, explicit false/empty owns a clear. Credentials
// still require the independent exact-serialization full-replacement guard.
var configOwnershipTypes = map[string][]string{
	"aws":              {"use_role_arn", "aws_role_arn", "use_service_account", "service_account", "create_if_not_exists", "enable_allowed_commands", "allowed_commands", "region_name"},
	"keyspaces":        {"use_role_arn", "aws_role_arn", "use_service_account", "service_account", "create_if_not_exists", "aws_region_name", "version", "keyspace", "keyspaces_endpoint"},
	"mysql":            {"ssl_mode", "use_rds_iam", "use_irsa", "auth_mode", "aws_region", "aws_role_arn", "aws_access_key_id", "aws_secret_access_key", "aws_service_account", "root_cert", "client_cert", "client_key", "old_version"},
	"postgres":         {"use_rds_iam", "use_irsa", "auth_mode", "aws_region", "aws_role_arn", "aws_access_key_id", "aws_secret_access_key", "aws_service_account", "ssl_mode", "tls_root_cert", "tls_cert_file", "tls_key_file"},
	"kafka":            {"use_msk_iam", "use_irsa", "aws_region", "aws_role_arn", "aws_access_key_id", "aws_secret_access_key", "aws_service_account", "bootstrap_servers"},
	"awselasticcache":  {"use_iam_auth", "use_irsa", "aws_role_arn", "aws_service_account", "aws_region_name", "aws_access_key_id", "aws_secret_access_key", "cache_name", "cache_type", "tls_enabled", "tls_skip_verify"},
	"mongodb":          {"client_certificate", "use_tls", "version"},
	"awsdocumentdb":    {"tls_enabled"},
	"syslog":           {"tls_enabled", "tls_ca_cert", "client_certificate", "client_key", "tls_skip_verify"},
	"services":         {"tls_enabled"},
	"sql_server":       {"ssl_mode"},
	"azuresqlserver":   {"ssl_mode"},
	"google":           {"service_account", "admin_email"},
	"rdp_windows":      {"allow_file_transfer"},
	"aruba_instant_on": nil, "aruba_sw": nil, "cisco_ngfw": nil, "fortinet_ngfw": nil, "fortinet_analyzer": nil, "fortinet_manager": nil, "hpe_switch": nil, "ivanti": nil, "juniper_sw": nil, "paloalto_ngfw": nil, "sophos_fw": nil,
}

func managedDeviceType(typ string) bool {
	switch typ {
	case "aruba_instant_on", "aruba_sw", "cisco_ngfw", "fortinet_ngfw", "fortinet_analyzer", "fortinet_manager", "hpe_switch", "ivanti", "juniper_sw", "paloalto_ngfw", "sophos_fw":
		return true
	}
	return false
}
func configOwnershipAttrs(typ string) []string {
	if managedDeviceType(typ) {
		login := "login_url"
		if typ == "cisco_ngfw" || typ == "fortinet_ngfw" || typ == "hpe_switch" {
			login = "uri"
		}
		return []string{"automation_mode", "fields", "script", "key", "port", "webui_port", login}
	}
	return configOwnershipTypes[typ]
}
