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
	for _, control := range []struct {
		attr    string
		allowed bool
	}{
		{"use_role_arn", typ == "aws" || typ == "keyspaces"},
		{"enable_allowed_commands", typ == "aws"},
		{"allowed_commands", typ == "aws"},
		{"keyspace", typ == "keyspaces"},
		{"keyspaces_endpoint", typ == "keyspaces"},
	} {
		if !control.allowed {
			v := get(control.attr)
			if v == true || (v != nil && v != false && v != "") {
				return fmt.Errorf("%s is not supported for resource type %q", control.attr, typ)
			}
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

func resourceContractCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if !d.NewValueKnown("type") {
		return nil
	}
	typ := d.Get("type").(string)
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
	return validateResourceContractInputs(typ, d.Get)
}

// validateCloudConfigOwnership fails closed for legacy SDK callers without raw
// configuration: omission suppression cannot distinguish their clears from
// unowned values, and a full replacement must not disable observed auth/policy.
func validateCloudConfigOwnership(typ string, existing bool, raw cty.Value) error {
	if existing && (typ == "aws" || typ == "keyspaces") && (raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType()) {
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
	for _, attr := range []string{"use_role_arn", "aws_role_arn", "use_service_account", "service_account", "create_if_not_exists", "enable_allowed_commands", "allowed_commands", "region_name", "aws_region_name", "version", "keyspace", "keyspaces_endpoint"} {
		r.Schema[attr].DiffSuppressFunc = cloudConfigOmissionDiffSuppress
	}
	return r
}

func cloudConfigOmissionDiffSuppress(attr, _, _ string, d *schema.ResourceData) bool {
	typ, _ := d.Get("type").(string)
	if typ != "aws" && typ != "keyspaces" {
		return false
	}
	switch attr {
	case "region_name", "enable_allowed_commands", "allowed_commands":
		if typ != "aws" {
			return false
		}
	case "aws_region_name", "version", "keyspace", "keyspaces_endpoint":
		if typ != "keyspaces" {
			return false
		}
	}
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() {
		return false
	}
	return !raw.Type().HasAttribute(attr) || raw.GetAttr(attr).IsNull()
}
