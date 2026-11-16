package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// ManagedBrowserSettings matches chrome.ManagedBrowserSettings, intentionally
// without prestart: managed network devices do not support that execution hook.
type ManagedBrowserSettings struct {
	AutomationMode string  `yaml:"automationMode,omitempty"`
	Fields         *string `yaml:"fields,omitempty"`
	Script         *string `yaml:"script,omitempty"`
}

func managedBrowserSettings(d *schema.ResourceData) ManagedBrowserSettings {
	return ManagedBrowserSettings{AutomationMode: attrString(d, "automation_mode"), Fields: managedBrowserString(d, "fields"), Script: managedBrowserString(d, "script")}
}

// managedBrowserString retains explicit empty clears in stored YAML so the
// subsequent read can acknowledge them and prune old secret fingerprints.
// Raw configuration establishes presence only; values always follow the plan.
func managedBrowserString(d *schema.ResourceData, attr string) *string {
	value := attrString(d, attr)
	raw := d.GetRawConfig()
	if raw.IsKnown() && !raw.IsNull() && raw.Type().IsObjectType() {
		if raw.Type().HasAttribute(attr) && !raw.GetAttr(attr).IsNull() || value != "" {
			return &value
		}
		return nil
	}
	if _, present := d.GetOkExists(attr); present || value != "" {
		return &value
	}
	return nil
}

func (a *configApplier) applyManagedBrowserSettings() {
	a.cloudString("automationMode", "automation_mode")
	a.cloudString("fields", "fields")
	a.cloudString("script", "script")
}
