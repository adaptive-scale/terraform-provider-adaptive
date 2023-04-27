package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type KubernetesIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	ApiServer    string `yaml:"apiserver"`
	ClusterToken string `yaml:"token"`
	ClusterCerts string `yaml:"cacrt"`
}

func schemaToKubernetesIntegrationConfiguration(d *schema.ResourceData) KubernetesIntegrationConfiguration {
	return KubernetesIntegrationConfiguration{
		Name:         d.Get("name").(string),
		ApiServer:    d.Get("api_server").(string),
		ClusterCerts: d.Get("cluster_cert").(string),
		ClusterToken: d.Get("cluster_token").(string),
	}
}
