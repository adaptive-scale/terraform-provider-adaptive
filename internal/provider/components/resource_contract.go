package components

import (
	"context"
	"fmt"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// validateResourceContractInputs is shared by planning and mutation callbacks:
// flat-schema arguments must not silently claim unsupported backend behavior.
func validateResourceContractInputs(typ string, get func(string) interface{}) error {
	if typ == "clickhouse" {
		mode, _ := get("ssl_mode").(string)
		if mode != "" {
			return fmt.Errorf("ClickHouse does not support ssl_mode; remove it and configure the use_tls boolean explicitly instead")
		}
	}
	if typ == "aruba_instant_on" {
		host, _ := get("host").(string)
		hostname, _ := get("hostname").(string)
		token, _ := get("api_token").(string)
		return integrations.ValidateArubaInstantOnInputs(host, hostname, token)
	}
	return nil
}

func resourceContractCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if !d.NewValueKnown("type") {
		return nil
	}
	typ := d.Get("type").(string)
	if typ == "clickhouse" && !d.NewValueKnown("ssl_mode") {
		return nil
	}
	if typ == "aruba_instant_on" {
		for _, attr := range []string{"host", "hostname", "api_token"} {
			if !d.NewValueKnown(attr) {
				return nil
			}
		}
	}
	return validateResourceContractInputs(typ, d.Get)
}
