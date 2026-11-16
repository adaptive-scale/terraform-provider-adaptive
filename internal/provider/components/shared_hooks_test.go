package components

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestAuthorizationMissingResourceTypeRefreshAndImport(t *testing.T) {
	for _, imported := range []bool{false, true} {
		t.Run(fmt.Sprint("import=", imported), func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"id":"a-1","name":"role","legacyPermission":"SELECT","status":"created"}`)
			})
			values := map[string]interface{}{}
			if !imported {
				values = map[string]interface{}{"name": "role", "resource_type": "postgres", "permissions": "SELECT"}
			}
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveAuthorization().Schema, values)
			d.SetId("a-1")
			diags := ResourceAdaptiveAuthorizationRead(context.Background(), d, client)
			if d.Id() != "a-1" {
				t.Fatal("lost identity")
			}
			if imported {
				if !diags.HasError() || !strings.Contains(fmt.Sprint(diags), "resource_type") {
					t.Fatalf("missing import type requires actionable error: %+v", diags)
				}
			} else {
				if diags.HasError() {
					t.Fatal(diags)
				}
				if got := d.Get("resource_type"); got != "postgres" {
					t.Fatalf("legacy omission erased prior type: %q", got)
				}
			}
		})
	}
}
