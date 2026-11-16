package components

import (
	"context"
	"fmt"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Keep existing contract/ownership checks and validate known RDP target leaves
// during planning too. Unknown expressions must remain resolvable at apply.
func rdpTargetCustomizeDiff(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
	if err := resourceContractCustomizeDiff(ctx, d, meta); err != nil {
		return err
	}
	targets, _ := d.Get("targets").([]interface{})
	for i := range targets {
		for _, field := range []string{"id", "host", "username", "port"} {
			key := fmt.Sprintf("targets.%d.%s", i, field)
			if !d.NewValueKnown(key) {
				continue
			}
			validate := integrations.ValidateAdaptiveRDPTargetString
			if field == "port" {
				validate = integrations.ValidateAdaptiveRDPTargetPort
			}
			if _, errs := validate(d.Get(key), key); len(errs) > 0 {
				return errs[0]
			}
		}
	}
	return nil
}
