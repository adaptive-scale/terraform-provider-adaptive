package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

// UpdateIntegration consumes name from YAML, not the JSON envelope:
// inventorize internal/service/integrations/integration_list.go:UpdateIntegration.
func TestIntegrationServerListUpdateCarriesConfigurationName(t *testing.T) {
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"fixture-id","name":"fleet","integrationType":"serverlist","configuration":{}}`))
			return
		}
		if r.URL.Path != "/api/v1/terraform/resource/update/fixture-id" {
			t.Errorf("unexpected mutation route: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writes++
		var request struct {
			Configuration   string   `json:"config"`
			UserTags        []string `json:"userTags"`
			IntegrationType string   `json:"integrationType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		var config map[string]interface{}
		if err := yaml.UnmarshalStrict([]byte(request.Configuration), &config); err != nil {
			t.Error(err)
			return
		}
		if config["name"] != "fleet" {
			t.Errorf("configuration.name = %v; want fleet", config["name"])
			http.Error(w, "resource name is required", http.StatusInternalServerError)
			return
		}
		for key, want := range map[string]interface{}{"version": "1", "hosts": "host-a\nhost-b", "user": "fixture-user", "password": "synthetic-password", "sshKey": "synthetic-key"} {
			if config[key] != want {
				t.Errorf("configuration.%s = %v; want %v", key, config[key], want)
			}
		}
		if request.IntegrationType != "serverlist" || !reflect.DeepEqual(request.UserTags, []string{"updated"}) {
			t.Errorf("wrong update envelope: %+v", request)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "fleet", "type": "serverlist", "hosts": []interface{}{"host-a", "host-b"}, "default_user": "fixture-user", "password": "synthetic-password", "key": "synthetic-key", "tags": []interface{}{"updated"},
	})
	d.SetId("fixture-id")
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, adaptive.NewClient("synthetic-token", srv.URL)); diags.HasError() {
		t.Fatalf("tags update failed: %v", diags)
	}
	if writes != 1 {
		t.Fatalf("writes = %d; want 1", writes)
	}
}
