package components

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestSecretBaselineSurvivesRefresh(t *testing.T) {
	d, _ := readWithDigests(t, map[string]interface{}{
		"name": "db", "type": "postgres", "host": "h", "username": "app",
		"password":        "configured-test-value",
		"applied_digests": map[string]interface{}{"password": "last-write"},
	}, "rotated")
	if got := stateStringMap(d, "applied_digests")["password"]; got != "last-write" {
		t.Fatal("refresh replaced the last-write fingerprint baseline")
	}
}

func TestSecretBaselineAndWarningSurviveRepeatedRefresh(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"unsupportedCredential":"rotated"}}`))
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "db", "type": "postgres",
		"applied_digests": map[string]interface{}{"unsupportedCredential": "last-write"},
	})
	d.SetId("r-1")
	for i := 0; i < 2; i++ {
		diags := ResourceAdaptiveResourceRead(context.Background(), d, client)
		if diags.HasError() || len(diags) != 1 {
			t.Fatal("unresolved structured or unmapped drift did not keep its warning")
		}
		if stateStringMap(d, "applied_digests")["unsupportedCredential"] != "last-write" {
			t.Fatal("refresh acknowledged warning-only drift without a write")
		}
	}
}

func TestSecretBaselineSurvivesMissingDigestResponse(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","configuration":{"hostname":"h"}}`))
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "db", "type": "postgres", "password": "synthetic-password",
		"applied_digests": map[string]interface{}{"password": "last-write"},
	})
	d.SetId("r-1")
	for i := 0; i < 2; i++ {
		if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); diags.HasError() {
			t.Fatal("valid legacy read failed")
		}
		if stateStringMap(d, "applied_digests")["password"] != "last-write" || d.Get("password").(string) != "synthetic-password" {
			t.Fatal("missing metadata erased credentials or last-write evidence")
		}
	}
}

func TestSecretBaselineDoesNotTreatNullAsClear(t *testing.T) {
	applied := map[string]string{"password": "last-write"}
	got := pruneClearedSecretDigests(applied, map[string]interface{}{"password": nil})
	if got["password"] != "last-write" {
		t.Fatal("null was mistaken for an explicitly cleared credential")
	}
}

func TestSecretBaselinePrunesAnExplicitServerClear(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","configuration":{"password":""}}`))
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "db", "type": "postgres", "password": "configured-test-value",
		"applied_digests": map[string]interface{}{"password": "last-write", "rootCert": "retained"},
	})
	d.SetId("r-1")
	if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); diags.HasError() {
		t.Fatal("read failed")
	}
	got := stateStringMap(d, "applied_digests")
	if _, ok := got["password"]; ok {
		t.Error("explicit server clear retained its stale secret baseline")
	}
	if got["rootCert"] != "retained" {
		t.Error("an absent key lost its secret baseline")
	}
	if d.Get("password").(string) != "" {
		t.Error("explicit server clear did not reach the credential attribute")
	}
}
