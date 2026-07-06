package components

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestSchemaToResourceIntegrationConfiguration_TargetsOnlyForAdaptiveRDP(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "win-1",
		"type":     "rdp_windows",
		"hostname": "10.0.1.10",
		"username": "administrator",
		"password": "secret",
		"targets": []interface{}{
			map[string]interface{}{"id": "t1", "host": "10.0.1.11", "username": "admin", "password": "pw"},
		},
	})

	_, err := schemaToResourceIntegrationConfiguration(d, "rdp_windows")
	if err == nil {
		t.Fatal("expected an error when `targets` is set on a non-adaptive_rdp resource")
	}
	if !strings.Contains(err.Error(), "adaptive_rdp") {
		t.Fatalf("expected the error to mention adaptive_rdp, got: %v", err)
	}
}

func TestSchemaToResourceIntegrationConfiguration_RDPWindowsEmptyPassword(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "win-1",
		"type":     "rdp_windows",
		"hostname": "10.0.1.10",
		"username": "administrator",
	})

	_, err := schemaToResourceIntegrationConfiguration(d, "rdp_windows")
	if err == nil {
		t.Fatal("expected an error for an empty rdp_windows password")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected a password error, got: %v", err)
	}
}

func TestSchemaToResourceIntegrationConfiguration_AdaptiveRDPTargetsAccepted(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "fleet-1",
		"type": "adaptive_rdp",
		"targets": []interface{}{
			map[string]interface{}{"id": "t1", "host": "10.0.1.11", "username": "admin", "password": "pw"},
		},
	})

	if _, err := schemaToResourceIntegrationConfiguration(d, "adaptive_rdp"); err != nil {
		t.Fatalf("unexpected error for adaptive_rdp with targets: %v", err)
	}
}
