package components

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type readCase struct {
	name string
	res  func() *schema.Resource
	read func(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics
	// identity is the required attribute ImportStatePassthroughContext leaves empty.
	identity string
}

func readCases() []readCase {
	return []readCase{
		{"resource", ResourceAdaptiveResource, ResourceAdaptiveResourceRead, "name"},
		{"session", ResourceAdaptiveSession, ResourceAdaptiveSessionRead, "name"},
		{"authorization", ResourceAdaptiveAuthorization, ResourceAdaptiveAuthorizationRead, "name"},
		{"team", ResourceAdaptiveTeam, ResourceAdaptiveTeamRead, "name"},
		{"script", ResourceAdaptiveScript, ResourceAdaptiveScriptRead, "name"},
		{"schedule", ResourceAdaptiveSchedule, ResourceAdaptiveScheduleRead, "name"},
		{"dataprotection", ResourceAdaptiveDataProtection, ResourceAdaptiveDataProtectionRead, "resource"},
	}
}

// A real 404 still clears imported IDs. A malformed success is instead a read
// error: the client cannot distinguish an unknown object from a broken server,
// and must not let either import or refresh adopt an empty object.
func TestImportMissingReadClearsID(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, notFound)
			d := schema.TestResourceDataRaw(t, tc.res().Schema, map[string]interface{}{})
			d.SetId("not-a-real-id")
			if diags := tc.read(context.Background(), d, client); diags.HasError() {
				t.Fatalf("read: %+v", diags)
			}
			if d.Id() != "" {
				t.Errorf("404 import must clear id, got %q", d.Id())
			}
		})
	}
}

func TestImportIdentitylessReadErrors(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			})
			d := schema.TestResourceDataRaw(t, tc.res().Schema, map[string]interface{}{})
			d.SetId("not-a-real-id")
			before := identityStateSnapshot(d)
			if diags := tc.read(context.Background(), d, client); !diags.HasError() {
				t.Error("malformed success must fail import, not adopt an empty object")
			}
			if after := identityStateSnapshot(d); !reflect.DeepEqual(before, after) {
				t.Errorf("malformed import changed state: before=%v after=%v", before, after)
			}
		})
	}
}

// Seed every schema attribute, including computed values, nested collections,
// and secret/digest fields. These synthetic values deliberately exercise state
// retention rather than configuration validation or a particular integration.
func nonemptyIdentityState(s *schema.Schema) interface{} {
	switch s.Type {
	case schema.TypeString:
		return "prior-value"
	case schema.TypeBool:
		return true
	case schema.TypeInt:
		return 7
	case schema.TypeFloat:
		return 1.5
	case schema.TypeMap:
		return map[string]interface{}{"prior-key": "prior-value"}
	case schema.TypeList, schema.TypeSet:
		switch elem := s.Elem.(type) {
		case *schema.Resource:
			values := map[string]interface{}{}
			for key, field := range elem.Schema {
				values[key] = nonemptyIdentityState(field)
			}
			return []interface{}{values}
		case *schema.Schema:
			return []interface{}{nonemptyIdentityState(elem)}
		}
	}
	panic("unsupported identity-test schema type")
}

func identityStateSnapshot(d *schema.ResourceData) map[string]string {
	out := map[string]string{}
	state := d.State()
	if state == nil {
		return out
	}
	for key, value := range state.Attributes {
		out[key] = value
	}
	return out
}

// Every malformed success must fail before the first state write, not merely
// preserve the ID while blanking names, settings, associations, or secrets.
func TestRefreshIdentitylessReadKeepsState(t *testing.T) {
	bodies := map[string]string{
		"empty":          `{}`,
		"null":           `null`,
		"empty-envelope": `{"data":{}}`,
		"id-only":        `{"id":"r-1"}`,
		"empty-name":     `{"id":"r-1","name":"","resourceName":""}`,
		"wrong-id":       `{"id":"somebody-else","name":"other","resourceName":"other"}`,
	}
	for _, tc := range readCases() {
		for bodyName, body := range bodies {
			for _, status := range []int{http.StatusOK, http.StatusAccepted} {
				t.Run(tc.name+"/"+bodyName+"/"+http.StatusText(status), func(t *testing.T) {
					api := &fakeAPI{}
					client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(status)
						_, _ = w.Write([]byte(body))
					})
					res := tc.res()
					d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})
					for key, field := range res.Schema {
						if err := d.Set(key, nonemptyIdentityState(field)); err != nil {
							t.Fatalf("seed %s: %v", key, err)
						}
					}
					d.SetId("r-1")
					before := identityStateSnapshot(d)
					if diags := tc.read(context.Background(), d, client); !diags.HasError() {
						t.Error("malformed success must return an error")
					}
					if after := identityStateSnapshot(d); !reflect.DeepEqual(before, after) {
						t.Errorf("malformed refresh changed full state: before=%v after=%v", before, after)
					}
				})
			}
		}
	}
}
