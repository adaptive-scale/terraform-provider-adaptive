package integrations

/*
Example resource usage:

resource "adaptive_aws" "example" {
	  aws_region_name = "us-east-1"
	  aws_access_key_id = "myaccesskey"
	  aws_secret_access_key = "kysecretaccesskey"
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

type AWSCLIIntegrationConfiguration struct {
	Name               string `yaml:"name"`
	Version            string `yaml:"version"`
	AWSRegionName      string `yaml:"aws_region_name"`
	AWSAccessKeyID     string `yaml:"aws_access_key_id"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key"`
}

func resourceAdaptiveAWS() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveAWSCreate,
		ReadContext:   resourceAdaptiveAWSRead,
		UpdateContext: resourceAdaptiveAWSUpdate,
		DeleteContext: resourceAdaptiveAWSDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "",
			},
			"region_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The AWS region name to use for AWS CLI integration.",
			},
			"access_key_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The AWS access key ID to use for AWS CLI integration.",
			},
			"secret_access_key": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The AWS secret access key to use for AWS CLI integration.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func SchemaToAWSIntegrationConfiguration(d *schema.ResourceData) AWSCLIIntegrationConfiguration {
	return AWSCLIIntegrationConfiguration{
		Version:            "1.0",
		Name:               d.Get("name").(string),
		AWSRegionName:      d.Get("region_name").(string),
		AWSAccessKeyID:     d.Get("access_key_id").(string),
		AWSSecretAccessKey: d.Get("secret_access_key").(string),
	}
}

func resourceAdaptiveAWSCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "aws", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveAWSRead(ctx, d, m)
	return nil
}

func resourceAdaptiveAWSRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveAWSUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "aws", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveAWSRead(ctx, d, m)
}

func resourceAdaptiveAWSDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
