package provider

import (
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ServerListIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Hosts       string `yaml:"hosts"`
	DefaultUser string `yaml:"user"`
	SshKey      string `yaml:"sshKey"`
	Password    string `yaml:"password"`
}

func schemaToServerListIntegrationConfiguration(d *schema.ResourceData) (ServerListIntegrationConfiguration, error) {

	var hosts []string
	if _hosts, ok := d.GetOk("hosts"); ok {
		if _, ok = _hosts.([]interface{}); !ok {
			// TODO: instead attempting to parse, should we just error out?
			return ServerListIntegrationConfiguration{}, errors.New("could not parse hosts")
		} else {
			for _, __host := range _hosts.([]interface{}) {
				hosts = append(hosts, __host.(string))
			}
		}
	}

	hostsNSV := strings.Join(hosts, "\n")

	var sshKey string
	if v, ok := d.GetOk("key"); ok {
		sshKey = v.(string)
	}

	var password string
	if v, ok := d.GetOk("password"); ok {
		password = v.(string)
	}

	var defaultUser string
	if v, ok := d.GetOk("default_user"); ok {
		defaultUser = v.(string)
	}

	return ServerListIntegrationConfiguration{
		Version:     "1",
		Hosts:       hostsNSV,
		SshKey:      sshKey,
		Password:    password,
		DefaultUser: defaultUser,
	}, nil
}
