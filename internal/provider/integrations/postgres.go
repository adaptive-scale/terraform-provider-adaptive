package integrations

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
	"fmt"
	"time"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func resourceAdaptivePostgres() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAdaptivePostgresCreate,
		ReadContext:   resourceAdaptivePostgresRead,
		UpdateContext: resourceAdaptivePostgresUpdate,
		DeleteContext: resourceAdaptivePostgresDelete,

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
				Optional:    true,
				Description: "The password to authenticate with the Postgres instance. Not used when `use_rds_iam` is set.",
			},
			"ssl_mode": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The SSL mode to use when connecting to the Postgres instance.",
			},
			"use_rds_iam": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Authenticate with short-lived AWS RDS IAM auth tokens instead of a stored password. The database user must be granted rds_iam, the RDS instance must have IAM authentication enabled, and `ssl_mode` cannot be `disable`.",
			},
			"use_irsa": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Mint RDS IAM tokens with `aws_service_account`, or with the platform's own IRSA / instance-profile identity when it is empty. Set to false to use `aws_access_key_id` / `aws_secret_access_key` instead. Only used when `use_rds_iam` is set.",
			},
			"aws_region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The AWS region used to mint RDS IAM tokens. Leave empty to auto-detect from an *.rds.amazonaws.com hostname.",
			},
			"aws_role_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "An IAM role to assume when minting RDS IAM tokens. With IRSA it overrides the service account's eks.amazonaws.com/role-arn annotation.",
			},
			"aws_access_key_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Static AWS access key id used to mint RDS IAM tokens. Requires `use_irsa = false`.",
			},
			"aws_secret_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Static AWS secret access key used to mint RDS IAM tokens. Requires `use_irsa = false`.",
			},
			"aws_service_account": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of a Kubernetes ServiceAccount in the session cluster annotated with eks.amazonaws.com/role-arn; the platform mints RDS IAM tokens on its behalf. Requires `use_irsa = true`.",
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
			"root_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The root SSL certificate for the Postgres instance in PEM format.",
			},
			"tls_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The client TLS certificate for the Postgres instance in PEM format.",
			},
			"tls_key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The client TLS key for the Postgres instance in PEM format.",
			},
		},
	}
}

type PostgresIntegrationConfiguration struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"databaseName"`
	HostName     string `yaml:"hostname"`
	Port         string `yaml:"port"`
	SSLMode      string `yaml:"sslMode"`
	TLSRootCert  string `yaml:"rootCert"`
	TLSCertFile  string `yaml:"crtText"`
	TLSKeyFile   string `yaml:"keyText"`

	// AWS RDS IAM authentication. These are only emitted when use_rds_iam is
	// set, so a password resource marshals exactly as it did before.
	UseRDSIAM          *bool  `yaml:"useRdsIam,omitempty"`
	UseIRSA            *bool  `yaml:"useIrsa,omitempty"`
	AWSRegion          string `yaml:"awsRegion,omitempty"`
	AWSRoleARN         string `yaml:"awsRoleArn,omitempty"`
	AWSAccessKeyID     string `yaml:"awsAccessKeyId,omitempty"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey,omitempty"`
	AWSServiceAccount  string `yaml:"awsServiceAccount,omitempty"`
}

// rdsIAMFromSchema reads the AWS RDS IAM block off a resource. It returns a
// zero value unless use_rds_iam is set, so password resources are untouched.
//
// use_irsa defaults to "no static keys were given", which matches how the
// platform infers the credential source, and an explicit value overrides it.
func rdsIAMFromSchema(d *schema.ResourceData) PostgresIntegrationConfiguration {
	useRDSIAM, _ := d.Get("use_rds_iam").(bool)
	if !useRDSIAM {
		return PostgresIntegrationConfiguration{}
	}

	accessKeyID, _ := d.Get("aws_access_key_id").(string)
	useIRSA := accessKeyID == ""
	if v, ok := d.GetOkExists("use_irsa"); ok {
		if b, ok := v.(bool); ok {
			useIRSA = b
		}
	}

	cfg := PostgresIntegrationConfiguration{
		UseRDSIAM:      &useRDSIAM,
		UseIRSA:        &useIRSA,
		AWSAccessKeyID: accessKeyID,
	}
	cfg.AWSRegion, _ = d.Get("aws_region").(string)
	cfg.AWSRoleARN, _ = d.Get("aws_role_arn").(string)
	cfg.AWSSecretAccessKey, _ = d.Get("aws_secret_access_key").(string)
	cfg.AWSServiceAccount, _ = d.Get("aws_service_account").(string)
	return cfg
}

// TODO: .(string) is assumption will cause problems
func SchemaToPostgresIntegrationConfiguration(d *schema.ResourceData) PostgresIntegrationConfiguration {
	sslMode := ""
	if d.Get("ssl_mode") != nil {
		if _sslMode, ok := d.Get("ssl_mode").(string); ok {
			sslMode = _sslMode
		} else if !ok {
			sslMode = ""
		}
	}

	tlsRootCert := ""
	if d.Get("tls_root_cert") != nil {
		if _tlsRootCert, ok := d.Get("tls_root_cert").(string); ok {
			tlsRootCert = _tlsRootCert
		}
	}

	tlsCertFile := ""
	if d.Get("tls_cert_file") != nil {
		if _tlsCertFile, ok := d.Get("tls_cert_file").(string); ok {
			tlsCertFile = _tlsCertFile
		}
	}

	tlsKeyFile := ""
	if d.Get("tls_key_file") != nil {
		if _tlsKeyFile, ok := d.Get("tls_key_file").(string); ok {
			tlsKeyFile = _tlsKeyFile
		}
	}

	cfg := rdsIAMFromSchema(d)
	cfg.Name = d.Get("name").(string)
	cfg.Username = d.Get("username").(string)
	cfg.Password = d.Get("password").(string)
	cfg.DatabaseName = d.Get("database_name").(string)
	cfg.HostName = d.Get("host").(string)
	cfg.Port = d.Get("port").(string)
	cfg.SSLMode = sslMode
	cfg.TLSRootCert = tlsRootCert
	cfg.TLSCertFile = tlsCertFile
	cfg.TLSKeyFile = tlsKeyFile
	return cfg
}

func resourceAdaptivePostgresCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToPostgresIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	userTags, err := TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.CreateResource(ctx, rName, "postgres", config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resp.ID)
	resourceAdaptivePostgresRead(ctx, d, m)
	return nil
}

func resourceAdaptivePostgresRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}

func resourceAdaptivePostgresUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)
	resourceID := d.Id()

	obj := SchemaToPostgresIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		return diag.FromErr(fmt.Errorf("provider error, could not marshal: %w", err))
	}

	userTags, err := TagsFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	defaultCluster, err := DefaultClusterFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, "postgres", config, userTags, defaultCluster)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("last_updated", time.Now())
	return resourceAdaptivePostgresRead(ctx, d, m)
}

func resourceAdaptivePostgresDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	resourceID := d.Id()
	client := m.(*adaptive.Client)
	_, err := client.DeleteResource(ctx, resourceID, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
