package components

import (
	"context"
	"fmt"
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
			// A nested path belongs to the enclosing configured collection. The
			// builder still validates its required credential leaves.
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
