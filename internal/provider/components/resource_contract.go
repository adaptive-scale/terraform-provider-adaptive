package components

import (
	"context"
	"fmt"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// validateResourceContractInputs is shared by planning and mutation callbacks:
// flat-schema arguments must not silently claim unsupported backend behavior.
func validateResourceContractInputs(typ string, get func(string) interface{}) error {
	for _, control := range resourceContractControls(typ) {
		if !control.allowed {
			v := get(control.attr)
			if v == true || (v != nil && v != false && v != "") {
				if control.attr == "login_url" {
					return fmt.Errorf("login_url is not a public input for %s; configure uri, which maps to the backend login_url, instead", typ)
				}
				return fmt.Errorf("%s is not supported for resource type %q", control.attr, typ)
			}
		}
	}
	for _, legacy := range []struct{ attr, replacement string }{
		{"app_id", "client_id for OAuth integrations"},
		{"database_account", "hostname for Snowflake"},
		{"database_username", "username"},
		{"database_password", "password"},
	} {
		if v, ok := get(legacy.attr).(string); ok && v != "" {
			return fmt.Errorf("%s has no supported integration mapping; remove it and configure %s instead", legacy.attr, legacy.replacement)
		}
	}
	if mode, _ := get("auth_mode").(string); mode != "" && mode != "password" && mode != "aws_rds_iam" {
		return fmt.Errorf("auth_mode must be password or aws_rds_iam")
	}
	if cacheType, _ := get("cache_type").(string); cacheType != "" && cacheType != "serverless" && cacheType != "replication_group" {
		return fmt.Errorf("cache_type must be serverless or replication_group")
	}
	if mode, _ := get("ssl_mode").(string); (mode == "false" || mode == "skip-verify") && typ != "sql_server" && typ != "azuresqlserver" {
		return fmt.Errorf("ssl_mode false and skip-verify are supported only for SQL Server and Azure SQL Server resources")
	}
	if mode, _ := get("ssl_mode").(string); (mode == "verify_identity" || mode == "verify_ca" || mode == "required" || mode == "preferred" || mode == "disabled") && typ != "mysql" {
		return fmt.Errorf("MySQL-specific ssl_mode is supported only for MySQL resources")
	}
	if typ == "mysql" {
		// The backend forwards this token literally to mysql --ssl-mode. Keep
		// only canonical modes, empty (omit the flag), and the historical
		// CLI-accepted require prefix; never rewrite PostgreSQL spellings.
		mode, _ := get("ssl_mode").(string)
		switch mode {
		case "", "verify_identity", "verify_ca", "required", "preferred", "disabled", "require":
		default:
			return fmt.Errorf("MySQL ssl_mode must be verify_identity, verify_ca, required, preferred, disabled, the legacy require prefix, or empty; got %q", mode)
		}
	}
	if typ == "clickhouse" {
		mode, _ := get("ssl_mode").(string)
		if mode != "" {
			return fmt.Errorf("ClickHouse does not support ssl_mode; remove it and configure the use_tls boolean explicitly instead")
		}
	}
	if typ == "aruba_instant_on" {
		host, _ := get("host").(string)
		hostname, _ := get("hostname").(string)
		token, _ := get("api_token").(string)
		return integrations.ValidateArubaInstantOnInputs(host, hostname, token)
	}
	return nil
}

// validateResourceTypeTransition rejects unsupported rebinding without planning
// destructive replacement of a resource that may have attached endpoints.
func validateResourceTypeTransition(id string, oldValue, newValue interface{}) error {
	oldType, _ := oldValue.(string)
	newType, _ := newValue.(string)
	if id != "" && oldType != "" && newType != "" && integrations.WireType(oldType) != integrations.WireType(newType) {
		return fmt.Errorf("cannot change resource integration type in place; create a distinct resource and explicitly migrate attached endpoints")
	}
	return nil
}

func resourceContractCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if !d.NewValueKnown("type") {
		return nil
	}
	typ := d.Get("type").(string)
	oldType, newType := d.GetChange("type")
	if err := validateResourceTypeTransition(d.Id(), oldType, newType); err != nil {
		return err
	}
	if err := validateResourceControlPresence(typ, d.GetRawConfig()); err != nil {
		return err
	}
	if err := validateCloudConfigOwnership(typ, d.Id() != "", d.GetRawConfig()); err != nil {
		return err
	}
	if typ == "clickhouse" && !d.NewValueKnown("ssl_mode") {
		return nil
	}
	if typ == "aruba_instant_on" {
		for _, attr := range []string{"host", "hostname", "api_token"} {
			if !d.NewValueKnown(attr) {
				return nil
			}
		}
	}
	return validateResourceContractInputs(typ, func(attr string) interface{} {
		if !d.NewValueKnown(attr) {
			return nil
		}
		return d.Get(attr)
	})
}

// validateCloudConfigOwnership fails closed for legacy SDK callers without raw
// configuration: omission suppression cannot distinguish their clears from
// unowned values, and a full replacement must not disable observed auth/policy.
func validateCloudConfigOwnership(typ string, existing bool, raw cty.Value) error {
	if existing && len(configOwnershipAttrs(typ)) > 0 && (raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType()) {
		return fmt.Errorf("cannot safely update %s configuration without Terraform raw configuration ownership; refresh and plan before applying", typ)
	}
	return nil
}

// withCloudConfigOwnership keeps observed nonsecret cloud configuration when
// omitted. Explicit false/empty still owns and resets it. These remain Optional,
// not globally Computed: service_account is a 1Password secret, aws_role_arn is
// also a Postgres IAM input, and computed strings fall back to old state on an
// empty-string plan. Never suppress refresh (remote changes must be observed).
// Held credential omission is independently rejected by the existing write guard.
func withCloudConfigOwnership(r *schema.Resource) *schema.Resource {
	for typ := range configOwnershipTypes {
		for _, attr := range configOwnershipAttrs(typ) {
			if r.Schema[attr] != nil {
				r.Schema[attr].DiffSuppressFunc = cloudConfigOmissionDiffSuppress
			}
		}
	}
	return r
}

func cloudConfigOmissionDiffSuppress(attr, _, _ string, d *schema.ResourceData) bool {
	typ, _ := d.Get("type").(string)
	owned := false
	for _, name := range configOwnershipAttrs(typ) {
		if name == attr {
			owned = true
			break
		}
	}
	if !owned {
		return false
	}
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() {
		return false
	}
	return !raw.Type().HasAttribute(attr) || raw.GetAttr(attr).IsNull()
}
