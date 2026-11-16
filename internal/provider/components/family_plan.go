package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// rejectExistingChange fails closed, rather than scheduling destructive replacement.
// In particular, replacing masking policies can turn off a newly adopted policy.
func rejectExistingChange(field, reason string) schema.CustomizeDiffFunc {
	return func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		if d.Id() != "" && d.HasChange(field) {
			return fmt.Errorf("%s cannot be updated: %s", field, reason)
		}
		return nil
	}
}

const authorizationResourceTypeUpdateBlocked = "the backend filters updates by the requested integration type and cannot persist a type transition; backend support is required"

func rejectAuthorizationResourceTypeChange(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.Id() != "" && d.NewValueKnown("resource_type") && d.HasChange("resource_type") {
		return fmt.Errorf("resource_type cannot be updated: %s", authorizationResourceTypeUpdateBlocked)
	}
	return nil
}

const scheduleTimezoneUpdateBlocked = "the backend Terraform schedule update contract does not persist timezone changes; backend support is required (no unsafe create/upsert replacement is attempted)"

const descriptionClearBlocked = "the backend update contract preserves empty descriptions; clearing requires backend support"
const scheduleDurationClearBlocked = "the backend update contract preserves an omitted max_access_time; zero is a real 0m TTL, not an unset duration; clearing requires backend support"

func familyPlan(checks ...schema.CustomizeDiffFunc) schema.CustomizeDiffFunc {
	return func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
		for _, check := range checks {
			if err := check(ctx, d, m); err != nil {
				return err
			}
		}
		return nil
	}
}

func familyInputPlan(required, lists []string) schema.CustomizeDiffFunc {
	return func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		return validateFamilyInputs(d.Get, d.NewValueKnown, required, lists)
	}
}

func validateFamilyInputs(get func(string) interface{}, known func(string) bool, required, lists []string) error {
	for _, field := range required {
		if known != nil && !known(field) {
			continue
		}
		value, ok := get(field).(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s cannot be empty or whitespace", field)
		}
	}
	for _, field := range lists {
		if known != nil && !known(field) {
			continue
		}
		for _, raw := range get(field).([]interface{}) {
			value, ok := raw.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s entries cannot be empty or whitespace", field)
			}
		}
	}
	return nil
}

func rejectExistingClear(field string, empty interface{}, reason string) schema.CustomizeDiffFunc {
	return func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		if d.Id() != "" && d.HasChange(field) && d.Get(field) == empty {
			return fmt.Errorf("%s cannot be cleared: %s", field, reason)
		}
		return nil
	}
}
