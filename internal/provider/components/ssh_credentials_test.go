package components

import (
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func TestSSHCredentialsUseIndependentArguments(t *testing.T) {
	for _, tc := range []struct{ password, key string }{
		{password: "synthetic-password"},
		{key: "synthetic-private-key"},
		{password: "synthetic-password", key: "synthetic-private-key"},
	} {
		d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
			"name": "ssh", "type": "ssh", "host": "host", "username": "user",
			"password": tc.password, "key": tc.key,
		})
		obj, err := schemaToResourceIntegrationConfiguration(d, "ssh")
		if err != nil {
			t.Fatal("could not construct SSH configuration")
		}
		data, err := yaml.Marshal(obj)
		if err != nil {
			t.Fatal("could not serialize SSH configuration")
		}
		var cfg map[string]interface{}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatal("could not decode SSH configuration")
		}
		if cfg["password"] != tc.password || cfg["sshKey"] != tc.key || cfg["usePassword"] != (tc.key == "") {
			t.Error("SSH password and key did not come from their independent arguments")
		}
	}
}

func TestSSHRefreshMapsPasswordWithoutChangingKey(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "ssh", "type": "ssh", "password": "previous", "key": "synthetic-private-key",
	})
	if err := integrations.ApplyIntegrationConfig(d, "ssh", map[string]interface{}{"password": "updated"}); err != nil {
		t.Fatal("SSH refresh failed")
	}
	if d.Get("password").(string) != "updated" || d.Get("key").(string) != "synthetic-private-key" {
		t.Error("SSH password refresh did not preserve independent credentials")
	}
	if attr, ok := integrations.AttrForConfigKey("ssh", "password"); !ok || attr != "password" {
		t.Error("SSH password fingerprint has no correct attribute mapping")
	}
}
