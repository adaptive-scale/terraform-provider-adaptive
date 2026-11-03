package components

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func TestDatadogApplicationKeyWireContract(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "monitor", "type": "datadog", "dd_site": "datadoghq.example.invalid",
		"dd_api_key": "synthetic-api-key", "app_key": "synthetic-application-key",
	})
	obj, err := schemaToResourceIntegrationConfiguration(d, "datadog")
	if err != nil {
		t.Fatal("could not build Datadog configuration")
	}
	wire, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal("could not encode Datadog configuration")
	}
	var backend struct {
		Name        string `yaml:"name"`
		Site        string `yaml:"dd_site"`
		API         string `yaml:"dd_api_key"`
		Application string `yaml:"dd_app_key"`
	}
	if err := yaml.UnmarshalStrict(wire, &backend); err != nil {
		t.Fatal("Datadog payload has incorrect backend keys/types")
	}
	if backend.API != "synthetic-api-key" || backend.Application != "synthetic-application-key" {
		t.Error("independent Datadog API/application key contract was not retained")
	}
	if !ResourceAdaptiveResource().Schema["app_key"].Sensitive {
		t.Error("application key must remain sensitive")
	}
}

func TestDatadogApplicationKeyRefreshContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  map[string]interface{}
		want string
	}{
		{"absent", map[string]interface{}{"dd_site": "datadoghq.example.invalid"}, "held-application"},
		{"null-api", map[string]interface{}{"dd_api_key": nil, "dd_app_key": nil}, "held-application"},
		{"null", map[string]interface{}{"dd_app_key": nil}, "held-application"},
		{"empty", map[string]interface{}{"dd_app_key": ""}, ""},
		{"supplied", map[string]interface{}{"dd_app_key": "observed-application"}, "observed-application"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
				"name": "monitor", "type": "datadog", "dd_api_key": "held-api", "app_key": "held-application",
			})
			if err := integrations.ApplyIntegrationConfig(d, "datadog", tc.cfg); err != nil {
				t.Fatal("could not map Datadog configuration")
			}
			if d.Get("app_key") != tc.want || d.Get("dd_api_key") != "held-api" {
				t.Error("Datadog independent credential presence/clear handling is incorrect")
			}
		})
	}
	if attr, ok := integrations.AttrForConfigKey("datadog", "dd_app_key"); !ok || attr != "app_key" {
		t.Error("application key fingerprint is not mapped to its public argument")
	}
}

func TestDatadogHeldApplicationKeyUpdateGuard(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		t.Run(map[bool]string{false: "omitted", true: "supplied"}[supplied], func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "r-1", "name": "monitor", "integrationType": "datadog",
					"redactedDigests": map[string]string{"dd_api_key": "held-api-digest", "dd_app_key": "held-application-digest"},
				})
			})
			raw := map[string]cty.Value{"dd_api_key": cty.StringVal("configured-api")}
			if supplied {
				raw["app_key"] = cty.StringVal("configured-application")
			}
			d := secretUpdateData(t, map[string]interface{}{
				"name": "monitor", "type": "datadog", "dd_api_key": "configured-api", "app_key": "configured-application",
			}, raw)
			diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
			if diags.HasError() == supplied || api.sawPath("/update/") != supplied {
				t.Error("held application-key payload decision was unsafe or rejected supplied credentials")
			}
		})
	}
}
