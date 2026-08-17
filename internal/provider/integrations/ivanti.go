package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type IvantiIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	UsePassword bool   `yaml:"usePassword"`
	Password    string `yaml:"password"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	WebUIPort   string `yaml:"webui_port"`
	LoginURL    string `yaml:"login_url"`
	UseProxy    bool   `yaml:"use_proxy"`
	SSHKey      string `yaml:"sshKey"`
}

func SchemaToIvantiIntegrationConfiguration(d *schema.ResourceData) IvantiIntegrationConfiguration {
	return IvantiIntegrationConfiguration{
		Version:     "1.0",
		Name:        attrString(d, "name"),
		Username:    attrString(d, "username"),
		UsePassword: attrString(d, "key") == "",
		Password:    attrString(d, "password"),
		HostName:    attrString(d, "hostname"),
		Port:        attrString(d, "port"),
		WebUIPort:   attrString(d, "webui_port"),
		LoginURL:    attrString(d, "login_url"),
		UseProxy:    attrBool(d, "use_proxy"),
		SSHKey:      attrString(d, "key"),
	}
}
