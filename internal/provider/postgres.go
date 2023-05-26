package provider

/*
Example resource usage:

resource "adaptive_postgres" "example" {
  name          = "mydatabase256789"
  host          = "myhost.example.com"
  port          = "5433"
  username      = "myuser"
  password      = "mypasswor2"
  ssl_mode      = "require"
  database_name = ""
}

*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type PostgresIntegrationConfiguration struct {
	Name               string `yaml:"name"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	DatabaseName       string `yaml:"databaseName"`
	HostName           string `yaml:"hostname"`
	Port               string `yaml:"port"`
	SSLMode            string `yaml:"sslMode"`
	PasswordSecretPath string `yaml:"passwordSecretPath"`
}

// TODO: .(string) is assumption will cause problems
func schemaToPostgresIntegrationConfiguration(d *schema.ResourceData) PostgresIntegrationConfiguration {
	return PostgresIntegrationConfiguration{
		Name:         d.Get("name").(string),
		Username:     d.Get("username").(string),
		Password:     d.Get("password").(string),
		DatabaseName: d.Get("database_name").(string),
		HostName:     d.Get("host").(string),
		Port:         d.Get("port").(string),
		// SSLMode:      d.Get("ssl_mode").(string),
		PasswordSecretPath: d.Get("password_secret_path").(string),
	}
}

// func resourceAdaptivePostgres() *schema.Resource {
// 	return &schema.Resource{
// 		CreateContext: resourceAdaptivePostgresCreate,
// 		ReadContext:   resourceAdaptivePostgresRead,
// 		UpdateContext: resourceAdaptivePostgresUpdate,
// 		DeleteContext: resourceAdaptivePostgresDelete,

// 		Schema: map[string]*schema.Schema{
// 			"name": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The name of the Postgres database to create.",
// 			},
// 			"host": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The hostname of the Postgres instance to connect to.",
// 			},
// 			"port": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The port number of the Postgres instance to connect to.",
// 			},
// 			"username": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The username to authenticate with the Postgres instance.",
// 			},
// 			"password": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The password to authenticate with the Postgres instance.",
// 			},
// 			"ssl_mode": {
// 				Type:        schema.TypeString,
// 				Required:    true,
// 				Description: "The SSL mode to use when connecting to the Postgres instance.",
// 			},
// 			"database_name": {
// 				Type:        schema.TypeString,
// 				Optional:    true,
// 				Default:     "",
// 				Description: "The name of the Postgres database to create. If not specified, the default database will be used.",
// 			},
// 			"last_updated": {
// 				Type:     schema.TypeString,
// 				Optional: true,
// 				Computed: true,
// 			},
// 		},
// 	}
// }

// func resourceAdaptivePostgresCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)

// 	obj := schemaToPostgresIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	rName, err := nameFromSchema(d)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}
// 	resp, err := client.CreateResource(ctx, rName, "postgres", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId(resp.ID)
// 	resourceAdaptivePostgresRead(ctx, d, m)
// 	return nil
// }

// func resourceAdaptivePostgresRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	return nil
// }

// func resourceAdaptivePostgresUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	client := m.(*adaptive.Client)
// 	resourceID := d.Id()

// 	obj := schemaToPostgresIntegrationConfiguration(d)
// 	config, err := yaml.Marshal(obj)
// 	if err != nil {
// 		err := errors.New("provider error, could not marshal")
// 		return diag.FromErr(err)
// 	}

// 	_, err = client.UpdateResource(resourceID, "postgres", config)
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.Set("last_updated", time.Now())
// 	return resourceAdaptivePostgresRead(ctx, d, m)
// }

// func resourceAdaptivePostgresDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
// 	resourceID := d.Id()
// 	client := m.(*adaptive.Client)
// 	_, err := client.DeleteResource(resourceID, d.Get("name").(string))
// 	if err != nil {
// 		return diag.FromErr(err)
// 	}

// 	d.SetId("")
// 	return nil
// }
