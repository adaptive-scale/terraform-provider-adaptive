package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type KubernetesIntegrationConfiguration struct {
	Name                   string `yaml:"name"`
	ApiServer              string `yaml:"apiserver"`
	ClusterToken           string `yaml:"token"`
	ClusterCerts           string `yaml:"cacrt"`
	TokenSecretPath        string `yaml:"tokenSecretPath"`
	ClusterCertsSecretPath string `yaml:"cacrtSecretPath"`
}

func schemaToKubernetesIntegrationConfiguration(d *schema.ResourceData) KubernetesIntegrationConfiguration {
	return KubernetesIntegrationConfiguration{
		Name:                   d.Get("name").(string),
		ApiServer:              d.Get("api_server").(string),
		ClusterCerts:           strings.TrimSpace(d.Get("cluster_cert").(string)),
		ClusterToken:           strings.TrimSpace(d.Get("cluster_token").(string)),
		TokenSecretPath:        strings.TrimSpace(d.Get("token_secret_path").(string)),
		ClusterCertsSecretPath: strings.TrimSpace(d.Get("cluster_certs_secret_path").(string)),
	}
}
