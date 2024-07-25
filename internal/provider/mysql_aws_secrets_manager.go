package provider

/*
   Example resource usage:

   resource "adaptive_mysql" "example" {
   	  name          = "mydatabase256789"
   	  database_name = ""
   	  host          = "myhost.example.com"
   	  port          = "5433"
   	  username      = "myuser"
   	  password      = "mypasswor2"
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

type MySQLAWSIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
}

func resourceAdaptiveMyAWSSQL() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveMySQLAWSCreate,
		ReadContext:   resourceAdaptiveMySQLAWSRead,
		UpdateContext: resourceAdaptiveMySQLAWSUpdate,
		DeleteContext: resourceAdaptiveMySQLAWSDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Integration name for MySQL.",
			},
			"arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "ARN for the Assumed Role.",
			},
			"region": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "AWS region of the secrets manager secret.",
			},
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Secret ID of the secrets manager secret.",
			},
		},
	}
}

// TODO: .(string) is assumption will cause problems
func schemaToMySQLAWSIntegrationConfiguration(d *schema.ResourceData) MySQLAWSIntegrationConfiguration {
	return MySQLAWSIntegrationConfiguration{
		Version:  "",
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
	}
}

func resourceAdaptiveMySQLAWSCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := schemaToMySQLAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := nameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "mysql", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveMySQLAWSRead(ctx, d, m)
	return nil
}

func resourceAdaptiveMySQLAWSRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveMySQLAWSUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := schemaToMySQLAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "mysql", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveMySQLAWSUpdate(ctx, d, m)
}

func resourceAdaptiveMySQLAWSDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
