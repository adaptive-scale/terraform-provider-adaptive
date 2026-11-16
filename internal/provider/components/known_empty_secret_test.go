package components

import (
	"context"
	"encoding/json"
	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestKnownEmptySecretFirstAddition(t *testing.T) {
	cfg := map[string]interface{}{"password": ""}
	digests := map[string]string{}
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fixture", "integrationType": "mysql", "configuration": cfg, "redactedDigests": digests})
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{"name": "fixture", "type": "mysql"})
	d.SetId("r-1")
	read := func() {
		t.Helper()
		if ds := ResourceAdaptiveResourceRead(context.Background(), d, client); ds.HasError() {
			t.Fatal(ds)
		}
	}
	read()
	if !knownEmptySecretKeys(d)["password"] {
		t.Fatal("explicit server empty did not establish per-key provenance")
	}
	cfg = map[string]interface{}{}
	digests = map[string]string{"password": "opaque-first-addition"}
	for cycle := 0; cycle < 2; cycle++ {
		read()
		if d.Get("password") != "opaque-first-addition" {
			t.Fatal("first addition lost actionable drift marker")
		}
		if len(stateStringMap(d, "applied_digests")) != 0 {
			t.Fatal("refresh seeded away first addition")
		}
		if !knownEmptySecretKeys(d)["password"] {
			t.Fatal("refresh acknowledged known-empty drift")
		}
	}
}

func TestKnownEmptySecretUnknownEvidence(t *testing.T) {
	for _, cfg := range []map[string]interface{}{{}, {"password": nil}, {"host": ""}, {"password": 123}} {
		t.Run("unknown", func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fixture", "integrationType": "mysql", "configuration": cfg, "redactedDigests": map[string]string{"password": ""}})
			})
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{"name": "fixture", "type": "mysql", "password": ""})
			d.SetId("r-1")
			before := d.State().DeepCopy()
			ds := ResourceAdaptiveResourceRead(context.Background(), d, client)
			if ds.HasError() {
				if !reflect.DeepEqual(before, d.State()) {
					t.Fatal("bad read changed state")
				}
				return
			}
			s, ok := d.Get("known_empty_secret_keys").([]interface{})
			if !ok {
				t.Fatal("missing additive computed provenance field")
			}
			if len(s) != 0 {
				t.Fatal("unknown or nonsensitive evidence established empty provenance")
			}
			if len(stateStringMap(d, "applied_digests")) != 0 {
				t.Fatal("empty server fingerprint recorded")
			}
		})
	}
}

func TestKnownEmptyPartialClearAndAcknowledgement(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{"name": "fixture", "type": "proxysql", "applied_digests": map[string]interface{}{"password": "old-password", "adminPassword": "old-admin"}})
	r := &adaptive.ResourceReadResponse{IntegrationType: "proxysql", Configuration: map[string]interface{}{"password": ""}, RedactedDigests: map[string]string{"adminPassword": "old-admin"}}
	if _, err := refreshResourceSecretEvidence(d, r); err != nil {
		t.Fatal(err)
	}
	if stateStringMap(d, "applied_digests")["adminPassword"] != "old-admin" || len(stateStringMap(d, "applied_digests")) != 1 {
		t.Fatal("partial clear lost unaffected baseline")
	}
	r.Configuration = map[string]interface{}{}
	r.RedactedDigests["password"] = "new-password"
	for i := 0; i < 2; i++ {
		keys, err := refreshResourceSecretEvidence(d, r)
		if err != nil || len(keys) != 1 || keys[0] != "password" {
			t.Fatal("partial-clear addition was not retained as drift")
		}
	}
	if err := captureResourceSecretDigests(d, r); err != nil {
		t.Fatal(err)
	}
	if knownEmptySecretKeys(d)["password"] || stateStringMap(d, "applied_digests")["password"] != "new-password" {
		t.Fatal("verified capture did not reconcile provenance")
	}
}

func TestKnownEmptyCaptureUnknownPreservesEvidence(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{"type": "mysql", "known_empty_secret_keys": []interface{}{"password"}, "applied_digests": map[string]interface{}{"other": "legacy"}})
	before := d.State().DeepCopy()
	if err := captureResourceSecretDigests(d, &adaptive.ResourceReadResponse{IntegrationType: "mysql", Configuration: map[string]interface{}{}, RedactedDigests: map[string]string{"password": ""}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, d.State()) {
		t.Fatal("unknown postwrite evidence acknowledged drift")
	}
}

func TestKnownEmptyRDPWarningOnlyAggregate(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fixture", "integrationType": "adaptive_rdp", "configuration": map[string]interface{}{}, "redactedDigests": map[string]string{"targets": "new-aggregate"}})
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{"name": "fixture", "type": "adaptive_rdp", "applied_digests": map[string]interface{}{"targets": "old-aggregate"}})
	d.SetId("r-1")
	targets := []interface{}{map[string]interface{}{"id": "target-one", "host": "windows.invalid", "username": "fixture", "password": "synthetic-target-password"}}
	if err := d.Set("targets", targets); err != nil {
		t.Fatal(err)
	}
	beforeTargets := d.Get("targets")
	for i := 0; i < 2; i++ {
		ds := ResourceAdaptiveResourceRead(context.Background(), d, client)
		if ds.HasError() || len(ds) != 1 || !strings.Contains(ds[0].Detail, "aggregate RDP configuration") || strings.Contains(ds[0].Detail, "provider bug") {
			t.Fatal("missing clear aggregate RDP warning")
		}
		if stateStringMap(d, "applied_digests")["targets"] != "old-aggregate" || !reflect.DeepEqual(beforeTargets, d.Get("targets")) {
			t.Fatal("aggregate warning changed baseline or target state")
		}
	}
}
