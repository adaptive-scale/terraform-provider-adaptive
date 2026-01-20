package integrations

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type KubernetesIntegrationConfiguration struct {
	Name              string `yaml:"name"`
	ApiServer         string `yaml:"apiserver"`
	ClusterToken      string `yaml:"token"`
	ClusterCerts      string `yaml:"cacrt"`
	Namespace         string `yaml:"namespace,omitempty"`
	TolerationsBytes  string `yaml:"tolerationsBytes,omitempty"`
	AnnotationsBytes  string `yaml:"annotationsBytes,omitempty"`
	NodeSelectorBytes string `yaml:"nodeSelectorBytes,omitempty"`
	NodeAffinityBytes string `yaml:"affinityBytes,omitempty"`
}

func SchemaToKubernetesIntegrationConfiguration(d *schema.ResourceData) KubernetesIntegrationConfiguration {
	tolerationsBytes := ""
	if v, ok := d.GetOk("tolerations"); ok {
		tolerationsBytes = v.(string)
	}

	annotationsBytes := ""
	if v, ok := d.GetOk("annotations"); ok {
		annotationsBytes = v.(string)
	}

	nodeSelectorBytes := ""
	if v, ok := d.GetOk("node_selector"); ok {
		nodeSelectorBytes = v.(string)
	}

	nodeAffinityBytes := ""
	if v, ok := d.GetOk("node_affinity"); ok {
		nodeAffinityBytes = v.(string)
	}

	return KubernetesIntegrationConfiguration{
		Name:              d.Get("name").(string),
		ApiServer:         d.Get("api_server").(string),
		ClusterCerts:      strings.TrimSpace(d.Get("cluster_cert").(string)),
		ClusterToken:      strings.TrimSpace(d.Get("cluster_token").(string)),
		Namespace:         d.Get("namespace").(string),
		TolerationsBytes:  tolerationsBytes,
		AnnotationsBytes:  annotationsBytes,
		NodeSelectorBytes: nodeSelectorBytes,
		NodeAffinityBytes: nodeAffinityBytes,
	}
}
