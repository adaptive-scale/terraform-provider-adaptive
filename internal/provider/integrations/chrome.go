package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ChromeIntegrationConfiguration struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	Prestart       string `yaml:"prestart,omitempty"`
	AutomationMode string `yaml:"automationMode,omitempty"`
	Fields         string `yaml:"fields,omitempty"`
	Script         string `yaml:"script,omitempty"`
}

func SchemaToChromeIntegrationConfiguration(d *schema.ResourceData) ChromeIntegrationConfiguration {
	return ChromeIntegrationConfiguration{
		Name:           attrString(d, "name"),
		URL:            attrString(d, "url"),
		Prestart:       attrString(d, "prestart"),
		AutomationMode: attrString(d, "automation_mode"),
		Fields:         attrString(d, "fields"),
		Script:         attrString(d, "script"),
	}
}
