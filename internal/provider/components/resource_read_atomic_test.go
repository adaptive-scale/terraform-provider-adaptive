package components

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceInvalidNestedConfigurationDoesNotPartiallyRefreshState(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"changed","integrationType":"adaptive_rdp","userTags":["changed"],"configuration":{"targets":"not-a-target-list"}}`))
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "fleet", "type": "adaptive_rdp", "tags": []interface{}{"original"},
		"applied_digests": map[string]interface{}{"targets": "original-fingerprint"},
		"targets": []interface{}{map[string]interface{}{
			"id": "target-1", "host": "host", "username": "user", "password": "synthetic-password",
		}},
	})
	d.SetId("r-1")
	before := d.State().DeepCopy()
	if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); !diags.HasError() {
		t.Fatal("invalid nested configuration was accepted")
	}
	if !reflect.DeepEqual(before, d.State()) {
		t.Fatal("failed configuration parsing partially overwrote resource state")
	}
}
