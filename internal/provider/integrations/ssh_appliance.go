package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// SSHApplianceIntegrationConfiguration is the shape both juniper_sw and
// sophos_fw send: a plain SSH login with no web UI.
type SSHApplianceIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	UsePassword bool   `yaml:"usePassword"`
	Password    string `yaml:"password"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	SSHKey      string `yaml:"sshKey"`
}

func schemaToSSHApplianceConfiguration(d *schema.ResourceData) SSHApplianceIntegrationConfiguration {
	key := attrString(d, "key")
	return SSHApplianceIntegrationConfiguration{
		Version:     "1.0",
		Name:        attrString(d, "name"),
		Username:    attrString(d, "username"),
		UsePassword: key == "",
		Password:    attrString(d, "password"),
		HostName:    attrString(d, "hostname"),
		Port:        attrString(d, "port"),
		SSHKey:      key,
	}
}
