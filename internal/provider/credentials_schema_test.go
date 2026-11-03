package provider

import (
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/components"
)

func TestCredentialBearingAttributesAreSensitive(t *testing.T) {
	for _, attr := range []string{"service_account", "value", "fields", "script", "prestart"} {
		t.Run(attr, func(t *testing.T) {
			if !components.ResourceAdaptiveResource().Schema[attr].Sensitive {
				t.Errorf("credential-bearing attribute %s is not sensitive", attr)
			}
		})
	}
}

func TestServiceTokenIsSensitive(t *testing.T) {
	if !New("test")().Schema["service_token"].Sensitive {
		t.Error("provider service token is not sensitive")
	}
}
