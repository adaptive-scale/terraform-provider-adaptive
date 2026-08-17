package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// FortinetApplianceIntegrationConfiguration is the shape both fortinet_analyzer
// and fortinet_manager send. Both appliances authenticate over SSH, so
// usePassword is derived from whether an SSH key was supplied rather than being
// an input of its own.
type FortinetApplianceIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	UsePassword bool   `yaml:"usePassword"`
	Password    string `yaml:"password"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	WebUIPort   string `yaml:"webui_port"`
	LoginURL    string `yaml:"login_url"`
	SSHKey      string `yaml:"sshKey"`
}

func schemaToFortinetApplianceConfiguration(d *schema.ResourceData) FortinetApplianceIntegrationConfiguration {
	key := attrString(d, "key")
	return FortinetApplianceIntegrationConfiguration{
		Version:     "1.0",
		Name:        attrString(d, "name"),
		Username:    attrString(d, "username"),
		UsePassword: key == "",
		Password:    attrString(d, "password"),
		HostName:    attrString(d, "hostname"),
		Port:        attrString(d, "port"),
		WebUIPort:   attrString(d, "webui_port"),
		LoginURL:    attrString(d, "login_url"),
		SSHKey:      key,
	}
}
