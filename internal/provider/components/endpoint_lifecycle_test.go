package components

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestEndpointCreatePreservesIDAndReportsPostCreateReadFailure(t *testing.T) {
	for _, stage := range []string{"readiness", "post-read", "post-status"} {
		t.Run(stage, func(t *testing.T) {
			reads := 0
			creates := 0
			api := &fakeAPI{}
			c := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/create") {
					creates++
					fmt.Fprint(w, `{"id":"ep-1"}`)
					return
				}
				reads++
				if stage == "post-status" && reads > 1 {
					fmt.Fprint(w, `{"id":"ep-1","name":"n","status":"failed"}`)
					return
				}
				if stage == "readiness" || reads > 1 {
					w.WriteHeader(403)
					return
				}
				fmt.Fprint(w, `{"id":"ep-1","name":"n","status":"created"}`)
			})
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveSession().Schema, map[string]interface{}{"name": "n", "resource": "r"})
			diags := ResourceAdaptiveSessionCreate(context.Background(), d, c)
			if !diags.HasError() {
				t.Fatal("failed create verification suppressed")
			}
			if d.Id() != "ep-1" {
				t.Fatal("partial create identity lost")
			}
			if creates != 1 {
				t.Fatal("create replayed")
			}
		})
	}
}
func TestAuthorizationPartialCreateIdentitySurvivesFailure(t *testing.T) {
	api := &fakeAPI{}
	c := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/create") {
			fmt.Fprint(w, `{"id":"auth-1"}`)
			return
		}
		fmt.Fprint(w, `{"id":"auth-1","name":"n","status":"failed"}`)
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveAuthorization().Schema, map[string]interface{}{"name": "n", "resource_type": "postgres", "permissions": "SELECT"})
	if diags := ResourceAdaptiveAuthorizationCreate(context.Background(), d, c); !diags.HasError() {
		t.Fatal("failed readiness suppressed")
	}
	if d.Id() != "auth-1" {
		t.Fatal("recoverable identity lost")
	}
}
