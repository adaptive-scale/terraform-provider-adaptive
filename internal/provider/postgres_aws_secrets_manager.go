package provider

/*
   Example resource usage:

   resource "adaptive_postgres" "example" {
     name          = "mydatabase256789"
     host          = "myhost.example.com"
     port          = "5433"
     username      = "myuser"
     password      = "mypasswor2"
     database_name = ""
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

func resourceAdaptivePostgresAWS() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptivePostgresAWSCreate,
		ReadContext:   resourceAdaptivePostgresAWSRead,
		UpdateContext: resourceAdaptivePostgresAWSUpdate,
		DeleteContext: resourceAdaptivePostgresAWSDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Postgres database to create.",
			},
			"host": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The hostname of the Postgres instance to connect to.",
			},
			"port": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The port number of the Postgres instance to connect to.",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The username to authenticate with the Postgres instance.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The password to authenticate with the Postgres instance.",
			},
			"ssl_mode": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The SSL mode to use when connecting to the Postgres instance.",
			},
			"database_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "The name of the Postgres database to create. If not specified, the default database will be used.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

type PostgresIntegrationAWSConfiguration struct {
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
}

// TODO: .(string) is assumption will cause problems
func schemaToPostgresAWSIntegrationConfiguration(d *schema.ResourceData) PostgresIntegrationAWSConfiguration {
	return PostgresIntegrationAWSConfiguration{
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
	}
}

func resourceAdaptivePostgresAWSCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToPostgresAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "postgres", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptivePostgresAWSRead(ctx, d, m)
	return nil
}

func resourceAdaptivePostgresAWSRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptivePostgresAWSUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToPostgresAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "postgres", config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptivePostgresAWSRead(ctx, d, m)
}

func resourceAdaptivePostgresAWSDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
