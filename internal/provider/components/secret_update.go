package components

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// validateResourceSecretUpdate fails closed before the full configuration write.
// Reads cannot recover held credentials, so omitted arguments must not silently
// erase them during a tags-only update or after import. This is an interim guard
// until the backend supports atomic preserve/replace/clear secret updates.
func validateResourceSecretUpdate(ctx context.Context, d *schema.ResourceData, client *adaptive.Client, serialized []byte) error {
	live, err := client.ReadResource(ctx, d.Id())
	if err != nil {
		return fmt.Errorf("cannot verify server-held credentials before updating resource: %w", err)
	}
	if live == nil {
		return fmt.Errorf("cannot update resource: it no longer exists; refresh state before applying")
	}
	if integrations.ProviderType(live.IntegrationType) != d.Get("type").(string) {
		return fmt.Errorf("cannot update resource with a different integration type")
	}
	// This integration stores an opaque blob that the read API cannot expose
	// as a configuration mapping, so it has no held-field/digest inventory.
	if integrations.ProviderType(live.IntegrationType) == "adaptiveencrypt" && !configHasAttr(d, "value") {
		return fmt.Errorf("cannot safely update an adaptiveencrypt resource without an explicitly configured value; the read API cannot recover its opaque contents")
	}

	held := pruneClearedSecretDigests(stateStringMap(d, "applied_digests"), live.Configuration)
	for key, digest := range live.RedactedDigests {
		held[key] = digest
	}
	for _, key := range live.RedactedKeys {
		held[key] = ""
	}
	// A missing digest is not proof that a credential is absent: older or
	// format-dependent backend classifiers can return sensitive values in clear.
	// Require explicit intent for every observed mapped sensitive string, not
	// only one integration's service-account field. Null is unknown, not an
	// authoritative clear; only an explicit empty string proves it was cleared.
	attributes := ResourceAdaptiveResource().Schema
	for key, value := range live.Configuration {
		attr, mapped := integrations.AttrForConfigKey(live.IntegrationType, key)
		if !mapped {
			continue
		}
		attribute := attributes[attr]
		if attribute != nil && attribute.Sensitive && attribute.Type == schema.TypeString && value != "" {
			held[key] = ""
		}
	}

	// Inspect the exact YAML that will be sent, not just the flat schema:
	// conditional builders may discard otherwise configured arguments.
	var outgoing map[string]interface{}
	if len(held) > 0 {
		if err := yaml.Unmarshal(serialized, &outgoing); err != nil {
			return fmt.Errorf("cannot safely verify server-held credentials in serialized configuration")
		}
	}

	missing := make(map[string]bool)
	unmapped := make(map[string]bool)
	raw := d.GetRawConfig()
	for key := range held {
		attr, ok := integrations.AttrForConfigKey(live.IntegrationType, key)
		if !ok {
			// Nested held paths require explicit ownership of the enclosing
			// collection; RDP's raw, planned, and embedded payload are checked below.
			root := strings.FieldsFunc(key, func(r rune) bool { return r == '.' || r == '[' })
			if len(root) > 0 && root[0] != key {
				attr, ok = integrations.AttrForConfigKey(live.IntegrationType, root[0])
			}
		}
		if !ok {
			unmapped[key] = true
			continue
		}
		if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute(attr) {
			missing[attr] = true
			continue
		}
		value := raw.GetAttr(attr)
		if value.IsNull() || !value.IsWhollyKnown() {
			missing[attr] = true
			continue
		}
		if integrations.ProviderType(live.IntegrationType) == "adaptive_rdp" && attr == "targets" {
			if !rdpConfiguredTargetsMatchPlan(value, d.Get(attr)) {
				return fmt.Errorf("cannot safely update resource: planned targets differ from their configured values (possibly due to lifecycle.ignore_changes), or required credential leaves are missing; reconcile targets and lifecycle settings before applying")
			}
			if !rdpSerializedTargetsMatchPlan(value, d.Get(attr), outgoing["targets"]) {
				return fmt.Errorf("cannot safely update resource: serialized targets do not retain the configured targets and credential arguments")
			}
		}
		// Presence alone is not authorization to serialize a different planned
		// value. In particular, ignore_changes can preserve a refresh marker.
		if configured, ok := d.Get(attr).(string); ok && value.Type() == cty.String && configured != value.AsString() {
			return fmt.Errorf("cannot safely update resource: planned %s differs from its configured value (possibly due to lifecycle.ignore_changes); a full configuration update cannot preserve the server-held credential; reconcile the argument and lifecycle settings before applying", attr)
		}
		if value.Type() == cty.String {
			sent, present := outgoing[key]
			// Optional strings tagged omitempty express an explicit clear by
			// omission in this full-replacement protocol. Nonempty inputs must
			// actually survive the integration builder and serialization.
			// Certificate/key-document builders already strip boundary whitespace.
			// Accept that existing normalization without changing the configured
			// or planned value, but never treat whitespace-only input as a clear.
			expected := value.AsString()
			retained := credentialPayloadRetained(live.IntegrationType, key, expected, sent)
			if (!present && expected != "") || (present && !retained) {
				return fmt.Errorf("cannot safely update resource: serialized configuration does not retain the configured %s argument; check the integration's conditional settings or explicitly clear a supported optional string with \"\"", attr)
			}
		}
	}
	if len(unmapped) > 0 {
		return fmt.Errorf("cannot safely update resource: server-held configuration keys %s have no supported credential mapping; this provider cannot preserve them during a full configuration update", strings.Join(sortedSecretKeys(unmapped), ", "))
	}
	if len(missing) > 0 {
		return fmt.Errorf("updating resource would clear server-held values for %s; provide the matching configuration arguments, or explicitly clear a supported optional string argument with \"\"", strings.Join(sortedSecretKeys(missing), ", "))
	}
	return nil
}

// rdpConfiguredTargetsMatchPlan compares the whole ordered collection, applying
// only schema defaults and SDK zero values for omitted optional arguments. In
// particular, an ignored password or a reordered target is not explicit intent.
func rdpConfiguredTargetsMatchPlan(raw cty.Value, planned interface{}) bool {
	if raw.IsNull() || !raw.IsWhollyKnown() || (!raw.Type().IsListType() && !raw.Type().IsTupleType()) {
		return false
	}
	targets, ok := planned.([]interface{})
	if !ok || len(targets) == 0 || raw.LengthInt() != len(targets) {
		return false
	}
	leaves := ResourceAdaptiveResource().Schema["targets"].Elem.(*schema.Resource).Schema
	for i, configured := range raw.AsValueSlice() {
		target, ok := targets[i].(map[string]interface{})
		if !ok || configured.IsNull() || !configured.Type().IsObjectType() {
			return false
		}
		for name, leaf := range leaves {
			value := cty.NilVal
			if configured.Type().HasAttribute(name) {
				value = configured.GetAttr(name)
			}
			expected := leaf.ZeroValue()
			if leaf.Default != nil {
				expected = leaf.Default
			}
			if value == cty.NilVal || value.IsNull() {
				if leaf.Required {
					return false
				}
			} else {
				switch {
				case value.Type() == cty.String && leaf.Type == schema.TypeString:
					expected = value.AsString()
				case value.Type() == cty.Bool && leaf.Type == schema.TypeBool:
					expected = value.True()
				case value.Type() == cty.Number && leaf.Type == schema.TypeInt:
					n, accuracy := value.AsBigFloat().Int64()
					if accuracy != 0 || int64(int(n)) != n {
						return false
					}
					expected = int(n)
				default:
					return false
				}
			}
			if name == "password" && expected == "" {
				return false // RDP has no supported explicit password clear.
			}
			if !reflect.DeepEqual(expected, target[name]) {
				return false
			}
		}
	}
	return true
}

// rdpSerializedTargetsMatchPlan runs only after the raw-to-plan check. Compare
// the embedded YAML structurally, preserving list order, exact credential bytes,
// omitted optional strings, and the record override's inherit/false/true states.
func rdpSerializedTargetsMatchPlan(raw cty.Value, planned, sent interface{}) bool {
	embedded, ok := sent.(string)
	if !ok {
		return false
	}
	var actual []map[string]interface{}
	if err := yaml.UnmarshalStrict([]byte(embedded), &actual); err != nil {
		return false
	}
	targets := planned.([]interface{})
	expected := make([]map[string]interface{}, len(targets))
	configured := raw.AsValueSlice()
	for i, item := range targets {
		target := item.(map[string]interface{})
		expected[i] = make(map[string]interface{})
		for key, value := range target {
			if key == "record" || ((key == "name" || key == "domain") && value == "") {
				continue
			}
			expected[i][key] = value
		}
		if configured[i].Type().HasAttribute("record") {
			record := configured[i].GetAttr("record")
			if !record.IsNull() {
				expected[i]["record"] = record.True()
			}
		}
	}
	return reflect.DeepEqual(expected, actual)
}

// credentialPayloadRetained permits only the boundary-whitespace normalization
// already performed by the named integration builders. Passwords and other
// arbitrary secrets remain byte-exact. Contract tests exercise every listed
// builder/key pair so this compatibility list cannot silently weaken validation.
func credentialPayloadRetained(integrationType, key, configured string, sent interface{}) bool {
	value, ok := sent.(string)
	if !ok {
		return false
	}
	if value == configured {
		return true
	}
	normalizes := false
	switch integrations.ProviderType(integrationType) {
	case "gcp":
		normalizes = key == "key_file"
	case "big_query":
		normalizes = key == "credential_json"
	case "kubernetes":
		normalizes = key == "token" || key == "cacrt"
	case "mongodb36":
		normalizes = key == "clientCertificate"
	case "proxysql":
		normalizes = key == "rootCert" || key == "clientCert" || key == "clientKey"
	case "cockroachdb":
		normalizes = key == "rootCert"
	case "cockroachdb_aws_secrets_manager":
		normalizes = key == "db_rootCert"
	case "redis":
		normalizes = key == "crtText" || key == "keyText" || key == "caText"
	}
	normalized := strings.TrimSpace(configured)
	return normalizes && normalized != "" && value == normalized
}

func sortedSecretKeys(keys map[string]bool) []string {
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
