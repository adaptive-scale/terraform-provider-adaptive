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
	"errors"
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

	return PostgresIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		SSLMode:      sslMode,
		TLSRootCert:  tlsRootCert,
		TLSCertFile:  tlsCertFile,
		TLSKeyFile:   tlsKeyFile,
	}
}

func resourceAdaptivePostgresCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	obj := SchemaToPostgresIntegrationConfiguration(d)
	config, err := yaml.Marshal(obj)
	if err != nil {
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	rName, err := NameFromSchema(d)
	if err != nil {
		return diag.FromErr(err)
	}
	resp, err := client.CreateResource(ctx, rName, "postgres", config, []string{})
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
		err := errors.New("provider error, could not marshal")
		return diag.FromErr(err)
	}

	_, err = client.UpdateResource(ctx, resourceID, "postgres", config, []string{})
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
