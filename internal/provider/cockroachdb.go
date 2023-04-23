// cockroachdb.go
package provider

/*
Example resource usage:

	resource "adaptive_cockraochdb" "example" {
		name          = "mydatabase256789"
		host          = "myhost.example.com"
		port          = "5433"
		username      = "myuser"
		password      = "mypasswor2"
		ssl_mode      = "require"
		database_name = ""
		root_cert	 =  ""
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

type CockroachDBIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
	RootCert     string `yaml:"rootCert"`
}

func resourceAdaptiveCockroachDB() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveCockroachDBCreate,
		ReadContext:   resourceAdaptiveCockroachDBRead,
		UpdateContext: resourceAdaptiveCockroachDBUpdate,
		DeleteContext: resourceAdaptiveCockroachDBDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the CockroachDB database to create.",
			},
			"host": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The hostname of the CockroachDB instance to connect to.",
			},
			"port": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The port number of the CockroachDB instance to connect to.",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The username to authenticate with the CockroachDB instance.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The password to authenticate with the CockroachDB instance.",
			},
			"ssl_mode": {
				Type: schema.TypeString,
				// Required:    true,
				Optional:    true,
				Default:     "verify-full",
				Description: "The SSL mode to use when connecting to the CockroachDB instance.",
			},
			"database_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "The name of the CockroachDB database to create. If not specified, the default database will be used.",
			},
			"root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "The root certificate to use for the CockroachDB instance.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func schemaToCockroachDBIntegrationConfiguration(d *schema.ResourceData) CockroachDBIntegrationConfiguration {
	return CockroachDBIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      d.Get("ssl_mode").(string),
		RootCert:     d.Get("root_cert").(string),
	}
}

func resourceAdaptiveCockroachDBCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToCockroachDBIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "cockroachdb", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveCockroachDBRead(ctx, d, m)
	return nil
}

func resourceAdaptiveCockroachDBRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveCockroachDBUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToCockroachDBIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "cockroachdb", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveCockroachDBRead(ctx, d, m)
}

func resourceAdaptiveCockroachDBDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
