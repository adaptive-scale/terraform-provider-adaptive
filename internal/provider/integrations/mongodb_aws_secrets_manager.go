package integrations

import (
	"context"
	"errors"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func resourceAdaptiveMongoAWS() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptiveMongoAWSCreate,
		ReadContext:   resourceAdaptiveMongoAWSRead,
		UpdateContext: resourceAdaptiveMongoAWSUpdate,
		DeleteContext: resourceAdaptiveMongoAWSDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the MongoDB instance to create.",
			},
			"arn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Amazon Resource Name (ARN) of the AWS IAM Role to be used for authentication.",
			},
			"region": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The AWS region where the MongoDB instance is hosted.",
			},
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The identifier of the AWS Secrets Manager secret that contains the credentials for accessing the MongoDB instance.",
			},
			"key": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The key within the AWS Secrets Manager secret that holds the MongoDB credentials.",
			},
		},
	}
}

type MongoDBAWSIntegrationConfiguration struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	ARN      string `yaml:"arn"`
	Region   string `yaml:"region"`
	SecretID string `yaml:"secret_id"`
	Key      string `yaml:"key"`
}

// TODO: .(string) is assumption will cause problems
func SchemaToMongoAWSIntegrationConfiguration(d *schema.ResourceData) MongoDBAWSIntegrationConfiguration {
	return MongoDBAWSIntegrationConfiguration{
		Name:     d.Get("name").(string),
		ARN:      d.Get("arn").(string),
		Region:   d.Get("region").(string),
		SecretID: d.Get("secret_id").(string),
		Key:      d.Get("key").(string),
	}
}

func resourceAdaptiveMongoAWSCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToMongoAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "mongodb", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptiveMongoAWSRead(ctx, d, m)
	return nil
}

func resourceAdaptiveMongoAWSRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptiveMongoAWSUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToMongoAWSIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(resourceID, "mongodb", config, []string{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptiveMongoAWSRead(ctx, d, m)
}

func resourceAdaptiveMongoAWSDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
