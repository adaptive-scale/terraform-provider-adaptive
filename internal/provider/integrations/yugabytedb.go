package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type YugabyteDBIntegrationConfiguration struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslMode"`
	RootCert string `yaml:"rootCert"`
	Port     string `yaml:"port"`
}

func SchemaToYugabyteDBIntegrationConfiguration(d *schema.ResourceData) YugabyteDBIntegrationConfiguration {
	sslMode := ""
	if v, ok := d.GetOk("ssl_mode"); ok {
		sslMode = v.(string)
	}

	rootCert := ""
	if v, ok := d.GetOk("root_cert"); ok {
		rootCert = v.(string)
	}

	return YugabyteDBIntegrationConfiguration{
		Name:     d.Get("name").(string),
		Hostname: d.Get("host").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
		SSLMode:  sslMode,
		RootCert: rootCert,
		Port:     d.Get("port").(string),
	}
}
