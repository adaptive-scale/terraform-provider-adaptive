package components

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"gopkg.in/yaml.v2"
)

func TestObservedUnclassifiedSecretUpdateRequiresExplicitInput(t *testing.T) {
	for _, integration := range []struct{ typ, key, attr string }{
		{"1password", "service_account", "service_account"},
		{"google", "serviceAccount", "service_account"},
		{"postgres", "password", "password"},
		{"azure", "clientSecret", "client_secret"},
		{"datadog", "dd_api_key", "dd_api_key"},
	} {
		for _, observed := range []interface{}{"synthetic-observed-secret-never-in-diagnostics", nil, ""} {
			for _, mode := range []string{"omitted", "null", "unknown", "resupply", "clear", "ignored"} {
				label := "nonempty"
				if observed == nil {
					label = "null"
				} else if observed == "" {
					label = "empty"
				}
				t.Run(integration.typ+"/"+label+"/"+mode, func(t *testing.T) {
					api := &fakeAPI{}
					client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"id": "r-1", "name": "fixture", "integrationType": integration.typ,
							"configuration": map[string]interface{}{integration.key: observed},
						})
					})
					raw := map[string]cty.Value{"type": cty.StringVal(integration.typ)}
					planned := ""
					switch mode {
					case "null":
						raw[integration.attr] = cty.NullVal(cty.String)
					case "unknown":
						raw[integration.attr] = cty.UnknownVal(cty.String)
					case "resupply":
						planned = "synthetic-explicit-replacement"
						raw[integration.attr] = cty.StringVal(planned)
					case "clear":
						raw[integration.attr] = cty.StringVal("")
					case "ignored":
						planned = "synthetic-drift-marker"
						raw[integration.attr] = cty.StringVal("synthetic-explicit-replacement")
					}
					d := secretUpdateData(t, map[string]interface{}{"name": "fixture", "type": integration.typ, integration.attr: planned}, raw)
					before := d.State().DeepCopy()
					serialized, err := yaml.Marshal(map[string]interface{}{integration.key: planned})
					if err != nil {
						t.Fatal(err)
					}
					err = validateResourceSecretUpdate(context.Background(), d, client, serialized)
					allow := observed == "" || mode == "resupply" || mode == "clear"
					if (err == nil) != allow {
						t.Fatalf("unexpected observed-credential guard decision: allow=%t", allow)
					}
					if !reflect.DeepEqual(before, d.State()) {
						t.Fatal("credential preflight changed state")
					}
					if err != nil {
						for _, secret := range []string{"synthetic-observed-secret-never-in-diagnostics", "synthetic-explicit-replacement", "synthetic-drift-marker"} {
							if strings.Contains(err.Error(), secret) {
								t.Fatal("credential preflight exposed a value")
							}
						}
					}
				})
			}
		}
	}
}

func TestOnePasswordUnclassifiedCredentialOmissionNeverReachesWrite(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"r-1","name":"fixture","integrationType":"1password","configuration":{"service_account":"synthetic-held-account"}}`))
		} else {
			_, _ = w.Write([]byte(`{"success":true}`))
		}
	})
	d := secretUpdateData(t, map[string]interface{}{"name": "fixture", "type": "1password", "tags": []interface{}{"changed"}}, map[string]cty.Value{"type": cty.StringVal("1password")})
	before := d.State().DeepCopy()
	diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
	if !diags.HasError() {
		t.Error("omission of an observed unclassified credential was accepted")
	}
	if api.sawPath("/update/") {
		t.Error("credential omission reached the mutation endpoint")
	}
	if !reflect.DeepEqual(before, d.State()) {
		t.Error("rejected omission changed state")
	}
}
