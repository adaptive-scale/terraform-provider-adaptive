package components

import (
	"sort"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func knownEmptySecretKeys(d *schema.ResourceData) map[string]bool {
	out := map[string]bool{}
	for _, key := range d.Get("known_empty_secret_keys").([]interface{}) {
		out[key.(string)] = true
	}
	return out
}

// Only explicit server empty scalar sensitive strings prove absence. In
// particular, held-key metadata, nulls and missing fingerprints prove nothing.
func observeEmptySecretKeys(known map[string]bool, r *adaptive.ResourceReadResponse) {
	for key, value := range r.Configuration {
		attr, ok := integrations.AttrForConfigKey(r.IntegrationType, key)
		if text, isString := value.(string); ok && isString && text == "" && isSecretStringAttr(attr) && r.RedactedDigests[key] == "" {
			known[key] = true
		}
	}
}

func setKnownEmptySecretKeys(d *schema.ResourceData, known map[string]bool) error {
	keys := make([]string, 0, len(known))
	for key := range known {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Use a deterministic computed list, not TypeSet: SDK v2.40 apply
	// normalization restores wholly-known planned sets, losing deletions made
	// by post-write capture. Actual Terraform engine tests cover this boundary.
	return d.Set("known_empty_secret_keys", keys)
}

// Refresh may establish empty provenance but never acknowledges first additions.
// Legacy wholly empty baselines still seed unknown keys; nonempty legacy
// baselines retain the existing new-key drift contract.
func refreshResourceSecretEvidence(d *schema.ResourceData, r *adaptive.ResourceReadResponse) ([]string, error) {
	applied := pruneClearedSecretDigests(stateStringMap(d, "applied_digests"), r.Configuration)
	known := knownEmptySecretKeys(d)
	observeEmptySecretKeys(known, r)
	drift := []string{}
	for key, digest := range r.RedactedDigests {
		if digest == "" {
			continue
		}
		if len(applied) > 0 || known[key] {
			if old, ok := applied[key]; !ok || old != digest {
				drift = append(drift, key)
			}
		}
	}
	if len(applied) == 0 {
		for key, digest := range r.RedactedDigests {
			if digest != "" && !known[key] {
				applied[key] = digest
			}
		}
	}
	sort.Strings(drift)
	if err := d.Set("applied_digests", applied); err != nil {
		return nil, err
	}
	return drift, setKnownEmptySecretKeys(d, known)
}
