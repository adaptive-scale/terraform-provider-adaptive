package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ArubaSWIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	SSHKey      string `yaml:"sshKey"`
	UsePassword bool   `yaml:"usePassword"`
	Port        string `yaml:"port"`
	WebUIPort   string `yaml:"webui_port"`
	LoginURL    string `yaml:"login_url"`

	ManagedBrowserSettings `yaml:",inline"`
	Name                   string `yaml:"name"`
	Hostname               string `yaml:"hostname"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
}

func SchemaToArubaSWIntegrationConfiguration(d *schema.ResourceData) ArubaSWIntegrationConfiguration {
	return ArubaSWIntegrationConfiguration{
		Version:     "1.0",
		SSHKey:      attrString(d, "key"),
		UsePassword: attrString(d, "key") == "",
		Port:        attrString(d, "port"),
		WebUIPort:   attrString(d, "webui_port"),
		LoginURL:    attrString(d, "login_url"),

		ManagedBrowserSettings: managedBrowserSettings(d),
		Name:                   d.Get("name").(string),
		Hostname:               d.Get("hostname").(string),
		Username:               d.Get("username").(string),
		Password:               d.Get("password").(string),
	}
}
