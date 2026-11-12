package components

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func readDiagnostics(d *schema.ResourceData, err error) diag.Diagnostics {
	if errors.Is(err, adaptive.ErrNotFound) {
		d.SetId("")
		return nil
	}
	if errors.Is(err, adaptive.ErrReadUnsupported) {
		return diag.Diagnostics{{Severity: diag.Warning, Summary: "Server does not support refresh for this resource", Detail: "Existing Terraform state has been preserved. Upgrade the Adaptive server to enable drift detection."}}
	}
	return diag.FromErr(err)
}

// The SDK's legacy diff can discard an explicit empty string. Consult the
// protocol configuration first so clearing an override is not mistaken for
// omission; refreshed computed values remain available through state.
func optionalValue(d *schema.ResourceData, key string) (interface{}, bool) {
	config := d.GetRawConfig()
	if !config.IsNull() && config.IsKnown() && config.Type().HasAttribute(key) {
		value := config.GetAttr(key)
		if !value.IsNull() && value.IsKnown() {
			return d.Get(key), true
		}
	}
	value, exists := d.GetOkExists(key)
	if entries, ok := value.(map[string]interface{}); ok && len(entries) == 0 {
		return value, false
	}
	return value, exists
}

// Legacy state may have no count entry for an empty computed collection. The
// SDK otherwise plans a write just to learn its size, even for computed maps.
func collectionBaseline(d *schema.ResourceData, key string, empty interface{}) error {
	if _, exists := d.GetOk(key); !exists {
		return d.Set(key, empty)
	}
	return nil
}

// Keep the existing list schema and ordering for membership-like collections.
// The API may return the same members in any order (tags come from a Go map).
func stableList(previous, incoming interface{}) interface{} {
	old, ok := previous.([]interface{})
	if !ok {
		return incoming
	}
	values, ok := incoming.([]interface{})
	if !ok {
		return incoming
	}
	remaining := make(map[string]int)
	for _, value := range values {
		s, ok := value.(string)
		if !ok {
			return incoming
		}
		remaining[s]++
	}
	out := make([]interface{}, 0, len(values))
	for _, value := range old {
		s, ok := value.(string)
		if ok && remaining[s] > 0 {
			out = append(out, s)
			remaining[s]--
		}
	}
	var added []string
	for s, count := range remaining {
		for i := 0; i < count; i++ {
			added = append(added, s)
		}
	}
	sort.Strings(added)
	for _, s := range added {
		out = append(out, s)
	}
	return out
}

func setResponseFields(d *schema.ResourceData, data map[string]interface{}, fields map[string][]string) diag.Diagnostics {
	for attribute, keys := range fields {
		value, exists := adaptive.ResponseValue(data, keys...)
		if !exists {
			continue
		}
		if attribute == "tags" {
			// Endpoint reads serialize a bare tag as "tag=". Retain the HCL
			// spelling when these represent the same empty value.
			if tags, ok := value.([]interface{}); ok {
				for i, tag := range tags {
					for _, old := range d.Get("tags").([]interface{}) {
						a, aok := tag.(string)
						b, bok := old.(string)
						if !aok || !bok {
							continue
						}
						if !strings.Contains(a, "=") {
							a += "="
						}
						if !strings.Contains(b, "=") {
							b += "="
						}
						if a == b {
							tags[i] = old
							break
						}
					}
				}
			}
		}
		value = stableList(d.Get(attribute), value)
		if old, ok := d.Get(attribute).(string); ok {
			if incoming, ok := value.(string); ok && equivalentString(attribute, old, incoming) {
				value = old
			}
		}
		if err := d.Set(attribute, value); err != nil {
			return diag.FromErr(fmt.Errorf("refreshing %s: %w", attribute, err))
		}
	}
	return nil
}

func equivalentString(attribute, old, incoming string) bool {
	if old == incoming {
		return true
	}
	switch attribute {
	case "ttl", "idle_timeout", "pause_timeout", "max_jit_request_duration":
		a, e1 := time.ParseDuration(old)
		b, e2 := time.ParseDuration(incoming)
		return e1 == nil && e2 == nil && a == b
	case "expires_at":
		a, e1 := time.Parse(time.RFC3339, old)
		b, e2 := time.Parse(time.RFC3339, incoming)
		return e1 == nil && e2 == nil && a.Equal(b)
	}
	return false
}

// Redaction paths may point inside a YAML/JSON string or a nested structure.
// Preserve the whole attribute in that case; a partial value would erase secrets.
func redactedAttribute(data map[string]interface{}, key string) bool {
	paths, _ := data["redactedKeys"].([]interface{})
	for _, path := range paths {
		p, _ := path.(string)
		if p == key || strings.HasPrefix(p, key+".") || strings.HasPrefix(p, key+"[") {
			return true
		}
	}
	return false
}

func refreshDigests(d *schema.ResourceData, attribute string, value interface{}) diag.Diagnostics {
	old := d.Get(attribute)
	if err := d.Set(attribute, value); err != nil {
		return diag.FromErr(err)
	}
	if reflect.ValueOf(old).IsValid() && !reflect.ValueOf(old).IsZero() {
		// Empty maps and strings are the baseline on the first refresh.
		if reflect.ValueOf(old).Len() > 0 && !reflect.DeepEqual(old, d.Get(attribute)) {
			return diag.Diagnostics{{Severity: diag.Warning, Summary: "A write-only value changed on the server", Detail: "Adaptive reported a changed secret or script digest. The configured value remains in Terraform state because the server does not return plaintext. Review the external change before updating this resource."}}
		}
	}
	return nil
}
