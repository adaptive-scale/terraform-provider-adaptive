package components

import (
	"fmt"
	"github.com/hashicorp/go-cty/cty"
)

type resourceControlCapability struct {
	attr    string
	allowed bool
}

func resourceContractControls(typ string) []resourceControlCapability {
	return []resourceControlCapability{
		{"use_role_arn", typ == "aws" || typ == "keyspaces"},
		{"version", typ == "keyspaces" || typ == "mongodb"},
		{"app_key", typ == "datadog"},
		{"login_url", typ != "cisco_ngfw" && typ != "fortinet_ngfw" && typ != "hpe_switch"},
		{"admin_email", typ == "google"},
		{"allow_file_transfer", typ == "rdp_windows"},
		{"automation_mode", typ == "chrome" || managedDeviceType(typ)},
		{"fields", typ == "chrome" || managedDeviceType(typ)},
		{"script", typ == "chrome" || managedDeviceType(typ)},
		{"prestart", typ == "chrome"},
		{"use_rds_iam", typ == "postgres" || typ == "mysql"},
		{"auth_mode", typ == "postgres" || typ == "mysql"},
		{"use_msk_iam", typ == "kafka"},
		{"bootstrap_servers", typ == "kafka"},
		{"use_iam_auth", typ == "awselasticcache"},
		{"cache_name", typ == "awselasticcache"},
		{"cache_type", typ == "awselasticcache"},
		{"use_irsa", typ == "postgres" || typ == "mysql" || typ == "kafka" || typ == "awselasticcache"},
		{"aws_region", typ == "postgres" || typ == "mysql" || typ == "kafka"},
		{"aws_role_arn", typ == "postgres" || typ == "mysql" || typ == "kafka" || typ == "awselasticcache" || typ == "aws" || typ == "keyspaces"},
		{"aws_service_account", typ == "postgres" || typ == "mysql" || typ == "kafka" || typ == "awselasticcache"},
		{"aws_access_key_id", typ == "postgres" || typ == "mysql" || typ == "kafka" || typ == "awselasticcache" || typ == "keyspaces"},
		{"aws_secret_access_key", typ == "postgres" || typ == "mysql" || typ == "kafka" || typ == "awselasticcache" || typ == "keyspaces"},
		{"enable_allowed_commands", typ == "aws"},
		{"allowed_commands", typ == "aws"},
		{"keyspace", typ == "keyspaces"},
		{"keyspaces_endpoint", typ == "keyspaces"},
	}
}

// Presence matters even for unsupported explicit false/empty. Do not inspect
// or print values, and defer unknown inputs until apply.
func validateResourceControlPresence(typ string, raw cty.Value) error {
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() {
		return nil
	}
	for _, control := range resourceContractControls(typ) {
		if control.allowed || !raw.Type().HasAttribute(control.attr) {
			continue
		}
		v := raw.GetAttr(control.attr)
		if !v.IsNull() && v.IsKnown() {
			if control.attr == "login_url" {
				return fmt.Errorf("login_url is not a public input for %s; configure uri, which maps to the backend login_url, instead", typ)
			}
			return fmt.Errorf("%s is not supported for resource type %q; remove the argument rather than configuring an empty or false value", control.attr, typ)
		}
	}
	return nil
}
