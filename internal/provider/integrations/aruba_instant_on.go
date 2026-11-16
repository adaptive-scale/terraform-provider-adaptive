package integrations

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ValidateArubaInstantOnInputs rejects ambiguous aliases and an unsupported
// credential instead of silently discarding it. Diagnostics never include values.
func ValidateArubaInstantOnInputs(host, hostname, apiToken string) error {
	if apiToken != "" {
		return fmt.Errorf("Aruba Instant On does not support api_token; remove api_token and configure username with password or key")
	}
	if host != "" && hostname != "" && host != hostname {
		return fmt.Errorf("Aruba Instant On host and hostname must match when both are set; configure only one alias or make their values equal")
	}
	return nil
}

// ArubaInstantOnIntegrationConfiguration matches the backend's consumed YAML.
// Cloud/API tokens are not part of that contract.
type ArubaInstantOnIntegrationConfiguration struct {
	ManagedBrowserSettings `yaml:",inline"`
	Version                string `yaml:"version"`
	Name                   string `yaml:"name"`
	Hostname               string `yaml:"hostname"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	SSHKey                 string `yaml:"sshKey"`
	UsePassword            bool   `yaml:"usePassword"`
	Port                   string `yaml:"port"`
	WebUIPort              string `yaml:"webui_port"`
	LoginURL               string `yaml:"login_url"`
}

// arubaAliasOwners uses configuration, not refreshed state, to avoid making a
// previously populated alias look like a second user input. With no config
// (import), host remains the backwards-compatible public default.
func arubaAliasOwners(d *schema.ResourceData) (host, hostname bool) {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() {
		// ReadResource has no configuration at the protocol boundary. Preserve
		// established managed aliases; only a fresh import defaults to host.
		h, _ := d.Get("host").(string)
		hn, _ := d.Get("hostname").(string)
		if hn != "" && h == "" {
			return false, true
		}
		if hn != "" && hn == h {
			return true, true
		}
		return true, false
	}
	present := func(attr string) bool {
		return raw.Type().HasAttribute(attr) && !raw.GetAttr(attr).IsNull()
	}
	host, hostname = present("host"), present("hostname")
	if host && hostname {
		// An explicitly empty second alias is not an additional hostname.
		nonempty := func(attr string) bool {
			v := raw.GetAttr(attr)
			return !v.IsKnown() || v.AsString() != ""
		}
		h, hn := nonempty("host"), nonempty("hostname")
		if h || hn {
			host, hostname = h, hn
		}
	}
	if !host && !hostname {
		host = true
	}
	return
}

func arubaInstantOnHostname(d *schema.ResourceData) string {
	_, hostname := arubaAliasOwners(d)
	if hostname {
		return d.Get("hostname").(string)
	}
	// Only fall back by value when configuration is unavailable (older
	// imported states and TestResourceDataRaw). An explicitly empty host owns
	// its value and must not resurrect a stale hostname alias from state.
	host := d.Get("host").(string)
	raw := d.GetRawConfig()
	if host == "" && (raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType()) {
		return d.Get("hostname").(string)
	}
	return host
}

func (a *configApplier) arubaInstantOnAliases() {
	if a.collect != nil {
		a.str("hostname", "host")
		a.str("host", "host")
		return
	}
	key := "hostname"
	if _, exists := a.cfg[key]; !exists {
		key = "host"
	}
	if a.cfg[key] == nil {
		return
	}
	value, ok := GetStr(a.cfg, key)
	if !ok {
		return
	}
	host, hostname := arubaAliasOwners(a.d)
	if host {
		a.set("host", value)
	} else {
		a.set("host", "")
	}
	if hostname {
		a.set("hostname", value)
	} else {
		a.set("hostname", "")
	}
}

func SchemaToArubaInstantOnIntegrationConfiguration(d *schema.ResourceData) ArubaInstantOnIntegrationConfiguration {
	return ArubaInstantOnIntegrationConfiguration{
		ManagedBrowserSettings: managedBrowserSettings(d),
		Version:                "1.0",
		Name:                   d.Get("name").(string),
		Hostname:               arubaInstantOnHostname(d),
		Username:               d.Get("username").(string),
		Password:               d.Get("password").(string),
		SSHKey:                 d.Get("key").(string),
		UsePassword:            d.Get("key").(string) == "",
		Port:                   d.Get("port").(string),
		WebUIPort:              d.Get("webui_port").(string),
		LoginURL:               d.Get("login_url").(string),
	}
}
