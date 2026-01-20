package integrations

/*
Example resource usage:

	resource "adaptive_ssh" "example" {
		name = "instance-name"
		username = "myuser"
		hostname = "myhost.example.com"
		port = "22"
		key = ""
	}
*/

import (
	"context"
	"errors"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

type SSHIntegrationConfiguration struct {
	Version     string `yaml:"version"`
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	UsePassword bool   `yaml:"usePassword"`
	Password    string `yaml:"password"`
	HostName    string `yaml:"hostname"`
	Port        string `yaml:"port"`
	SSHKey      string `yaml:"sshKey"`
}

func resourceAdaptiveSSH() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveSSHCreate,
		ReadContext:   resourceAdaptiveSSHRead,
		UpdateContext: resourceAdaptiveSSHUpdate,
		DeleteContext: resourceAdaptiveSSHDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the SSH instance to create.",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The username to authenticate with the SSH instance.",
			},
			"host": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The hostname of the SSH instance to connect to.",
			},
			"port": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The port number of the SSH instance to connect to.",
			},
			"password": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"key": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "The SSH key to use when connecting to the instance. If not specified, password authentication will be used.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func SchemaToSSHIntegrationConfiguration(d *schema.ResourceData) SSHIntegrationConfiguration {
	return SSHIntegrationConfiguration{
		Version:     "1.0",
		Name:        d.Get("name").(string),
		Username:    d.Get("username").(string),
		UsePassword: d.Get("key").(string) == "",
		Password:    d.Get("key").(string),
		HostName:    d.Get("host").(string),
		Port:        d.Get("port").(string),
		SSHKey:      d.Get("key").(string),
	}
}

func resourceAdaptiveSSHCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToSSHIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "ssh", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveSSHRead(ctx, d, m)
	return nil
}

func resourceAdaptiveSSHRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveSSHUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToSSHIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, "ssh", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveSSHRead(ctx, d, m)
}

func resourceAdaptiveSSHDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
