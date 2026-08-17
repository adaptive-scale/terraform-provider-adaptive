package components

// Helpers shared by the per-resource Read implementations.
//
// Refresh policy: a read must not introduce diffs of its own. Attributes the
// server reports authoritatively are written straight into state; set-valued
// attributes keep the configuration's ordering while the sets agree, and
// optional attributes the user never set are not adopted from the server.

import (
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// notFoundGone drops a resource that no longer exists from state so Terraform
// plans it for recreation instead of failing the refresh.
func notFoundGone(d *schema.ResourceData) diag.Diagnostics {
	d.SetId("")
	return nil
}

// isImportRead reports whether a Read is running against imported state.
// ImportStatePassthroughContext seeds state with nothing but the ID, so a
// required attribute reading back empty means nothing has been refreshed yet.
func isImportRead(d *schema.ResourceData, requiredAttr string) bool {
	v, ok := d.Get(requiredAttr).(string)
	return ok && v == ""
}

// stateStringSlice reads a TypeList or TypeSet of strings out of state,
// skipping empty entries.
func stateStringSlice(d *schema.ResourceData, key string) []string {
	switch raw := d.Get(key).(type) {
	case []interface{}:
		return toStringSlice(raw)
	case *schema.Set:
		return toStringSlice(raw.List())
	default:
		return nil
	}
}

func toStringSlice(raw []interface{}) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// stateStringMap reads a TypeMap of strings out of state.
func stateStringMap(d *schema.ResourceData, key string) map[string]string {
	raw, ok := d.Get(key).(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// sameStringMap reports whether two string maps hold the same entries.
func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// sameStringSet reports whether two string slices hold the same elements,
// ignoring order and duplicates.
func sameStringSet(a, b []string) bool {
	return sameSetBy(a, b, func(s string) string { return s })
}

// sameStringSetFold is sameStringSet with case-insensitive comparison, for
// attributes the server normalizes the case of (e.g. weekday names).
func sameStringSetFold(a, b []string) bool {
	return sameSetBy(a, b, strings.ToLower)
}

// sameInstantSet compares two lists of RFC3339 timestamps as instants, so
// equivalent spellings of the same moment ("2030-01-01T00:00:00Z" vs
// "2030-01-01T05:30:00+05:30") do not read as drift. Values that do not parse
// fall back to exact string comparison.
func sameInstantSet(a, b []string) bool {
	return sameSetBy(a, b, func(s string) string {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s
		}
		return t.UTC().Format(time.RFC3339)
	})
}

func sameSetBy(a, b []string, key func(string) string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	as := make(map[string]struct{}, len(a))
	for _, v := range a {
		as[key(v)] = struct{}{}
	}
	bs := make(map[string]struct{}, len(b))
	for _, v := range b {
		bs[key(v)] = struct{}{}
	}
	if len(as) != len(bs) {
		return false
	}
	for v := range as {
		if _, ok := bs[v]; !ok {
			return false
		}
	}
	return true
}

// setPreservingOrder reconciles a set-valued attribute: while the sets agree
// the configuration's ordering is kept, otherwise the server value wins.
func setPreservingOrder(prior, server []string) []string {
	return setPreservingOrderBy(prior, server, sameStringSet)
}

// setPreservingOrderBy is setPreservingOrder with a custom equality test.
func setPreservingOrderBy(prior, server []string, same func(a, b []string) bool) []string {
	if same(prior, server) {
		return prior
	}
	return server
}

// optString reconciles an optional string attribute: an attribute the user
// never set is not populated from the server (that would create a perpetual
// diff against an empty configuration), but a value the user did set is
// refreshed when the server disagrees. On import everything comes from the
// server.
func optString(prior, server string, isImport bool) string {
	if isImport {
		return server
	}
	if prior == "" {
		return ""
	}
	return server
}
