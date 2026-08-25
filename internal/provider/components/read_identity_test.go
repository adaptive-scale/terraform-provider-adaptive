package components

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// emptyObject answers every read with a 200 and a body that parses but names
// nothing — the shape a server would return for an unknown id if it answered
// with an empty body instead of 404.
func emptyObject(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`{}`))
}

type readCase struct {
	name string
	res  func() *schema.Resource
	read func(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics
	// identity is the required attribute that proves the object exists, and the
	// one ImportStatePassthroughContext leaves empty.
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

// An import whose read carries no identity must clear the id. Terraform turns a
// null state during import into "Cannot import non-existent remote object";
// leaving the id set would instead land an object of empty attributes in state
// under the id the user typed, and every later plan would diff against it.
func TestImportIdentitylessReadClearsID(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, emptyObject)

			// ImportStatePassthroughContext seeds nothing but the id.
			d := schema.TestResourceDataRaw(t, tc.res().Schema, map[string]interface{}{})
			d.SetId("not-a-real-id")

			if diags := tc.read(context.Background(), d, client); diags.HasError() {
				t.Fatalf("read: %+v", diags)
			}
			if d.Id() != "" {
				t.Errorf("id should be cleared so Terraform reports the object does not exist, got %q", d.Id())
			}
		})
	}
}

// The same identity-less response during a *refresh* must not drop the resource:
// state already holds the configuration's values, and deleting a live resource
// from state over a malformed read would be far worse than the stale read.
func TestRefreshIdentitylessReadKeepsState(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, emptyObject)

			d := schema.TestResourceDataRaw(t, tc.res().Schema, map[string]interface{}{
				tc.identity: "already-known",
			})
			d.SetId("r-1")

			if diags := tc.read(context.Background(), d, client); diags.HasError() {
				t.Fatalf("read: %+v", diags)
			}
			if d.Id() != "r-1" {
				t.Errorf("refresh must not drop the resource, id became %q", d.Id())
			}
		})
	}
}

// A read that describes a different object than the one requested is a fault,
// not drift — surfaced as an error rather than written into state.
func TestReadWrongRecordErrors(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":"somebody-else","name":"other","resourceName":"other"}`))
			})

			d := schema.TestResourceDataRaw(t, tc.res().Schema, map[string]interface{}{
				tc.identity: "already-known",
			})
			d.SetId("asked-for-this-one")

			diags := tc.read(context.Background(), d, client)
			if !diags.HasError() {
				t.Fatal("a read returning a different record must fail")
			}
		})
	}
}
