package components

import (
	"context"
	"fmt"
	"strings"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Refreshing adaptive_resource means inverting schemaToResourceIntegrationConfiguration:
// the forward direction folds ~80 schema attributes into a per-type YAML blob,
// and a read has to map the blob's keys back to the attributes that produced them.
//
// Two things keep that tractable. First, the backend strips every credential
// before returning the blob, so only the non-secret keys need an inverse - the
// secret ones are never refreshed at all, which is what we want anyway since
// they are write-only. Second, almost every remaining key maps 1:1 onto one
// attribute; only a handful are overloaded across integration types, and those
// are listed explicitly below.

// resourceConfigKeyToAttr is the type-independent inverse: a stored config key
// mapped to the schema attribute that wrote it.
var resourceConfigKeyToAttr = map[string]string{
	"hostname":     "host",
	"host":         "host",
	"port":         "port",
	"username":     "username",
	"databaseName": "database_name",
	"sslMode":      "ssl_mode",
	"uri":          "uri",
	"protocol":     "protocol",

	"apiserver": "api_server",
	"namespace": "namespace",

	"region":          "region",
	"project_id":      "project_id",
	"domain":          "domain",
	"clientID":        "client_id",
	"applicationID":   "application_id",
	"tenantID":        "tenant_id",
	"apiClientID":     "api_client_id",
	"organization_id": "organization_id",
	"public_key":      "public_key",

	"warehouse":  "warehouse",
	"schema":     "schema",
	"role":       "role",
	"clientcert": "clientcert",

	"urls":  "urls",
	"hosts": "hosts",
	"user":  "default_user",
	"index": "index",

	"dd_site":              "dd_site",
	"image":                "image",
	"service_account_name": "service_account_name",
	"network_id":           "network_id",
	"webui_port":           "webui_port",
	"use_proxy":            "use_proxy",
	"use_service_account":  "use_service_account",
	"create_if_not_exists": "create_if_not_exists",
	"useTenant":            "use_tenant",
	"applicationName":      "application_name",
	"subSystemName":        "sub_system_name",
	"tokenID":              "token_id",

	"arn":       "arn",
	"secret_id": "secret_id",
	"aws_arn":   "aws_arn",

	"useRdsIam":         "use_rds_iam",
	"useIrsa":           "use_irsa",
	"awsRegion":         "aws_region",
	"awsRoleArn":        "aws_role_arn",
	"awsAccessKeyId":    "aws_access_key_id",
	"awsServiceAccount": "aws_service_account",
}

// resourceConfigKeyOverrides lists the keys whose meaning depends on the
// integration type. Each entry wins over resourceConfigKeyToAttr.
//
// These are the cases where the forward mappers deliberately reuse one config
// key for different attributes - snowflake stores `host` under databaseAccount,
// the firewall integrations put their console address in login_url while
// paloalto_ngfw keeps a separate login_url attribute, and so on. Getting these
// wrong would not fail loudly; it would write a value into the wrong attribute
// and show drift forever.
var resourceConfigKeyOverrides = map[string]map[string]string{
	"snowflake": {
		"databaseAccount":  "hostname",
		"databaseUsername": "username",
		"warehouse":        "warehouse",
	},

	// These types write the endpoint address under `hostname` from the
	// `hostname` attribute, not from `host`.
	"aruba_sw":         {"hostname": "hostname"},
	"azuresqlserver":   {"hostname": "hostname"},
	"cisco_ngfw":       {"hostname": "hostname", "login_url": "uri"},
	"fortinet_ngfw":    {"hostname": "hostname", "login_url": "uri"},
	"hpe_switch":       {"hostname": "hostname", "login_url": "uri"},
	"paloalto_ngfw":    {"hostname": "hostname", "login_url": "login_url"},
	"syslog":           {"hostname": "hostname"},
	"rdp_windows":      {"hostname": "hostname"},
	"aruba_instant_on": {"host": "host"},

	// `url` is the resource uri for these, but a distinct `url` attribute for splunk.
	"coralogix":           {"url": "uri"},
	"elasticsearch":       {"url": "uri"},
	"custom_siem_webhook": {"url": "uri"},
	"rabbitmq":            {"url": "uri"},
	"splunk":              {"url": "url"},

	"azurecosmosnosql": {"endpoint": "uri"},

	// aws uses aws_region_name for region_name; awssecretsmanager has its own
	// aws_region_name attribute.
	"aws":               {"aws_region_name": "region_name"},
	"awssecretsmanager": {"aws_region_name": "aws_region_name"},
}

// resourceConfigAttr resolves one stored config key to a schema attribute for a
// given integration type. ok is false when the key has no inverse - either it is
// a key the provider never wrote (the backend stamps `version` into every blob)
// or one whose value is a secret the read deliberately withheld.
func resourceConfigAttr(integrationType, key string) (string, bool) {
	if overrides, ok := resourceConfigKeyOverrides[integrationType]; ok {
		if attr, ok := overrides[key]; ok {
			return attr, true
		}
	}
	attr, ok := resourceConfigKeyToAttr[key]
	return attr, ok
}

func ResourceAdaptiveResourceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*adaptive.Client)

	resource, err := client.GetResource(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	// Deleted out of band — drop it so Terraform plans a recreate.
	if resource == nil {
		d.SetId("")
		return nil
	}

	// Envelope fields are stored in their own columns, so they are unambiguous.
	if err := d.Set("name", resource.Name); err != nil {
		return diag.FromErr(err)
	}
	// The backend rewrites "services" to "servicelist" on the way in; map it back
	// so a config declaring type = "services" does not show drift.
	integrationType := resource.IntegrationType
	if integrationType == "servicelist" {
		integrationType = "services"
	}
	if err := d.Set("type", integrationType); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("tags", resource.UserTags); err != nil {
		return diag.FromErr(err)
	}
	// default_cluster is written as a cluster name and stored as an id; the
	// backend resolves it back to the name for us.
	if resource.DefaultCluster != "" {
		if err := d.Set("default_cluster", resource.DefaultCluster); err != nil {
			return diag.FromErr(err)
		}
	}

	// Attributes whose values were withheld must keep their prior state. Setting
	// them from an absent value would propose clearing a live credential.
	withheld := make(map[string]bool, len(resource.RedactedKeys))
	for _, key := range resource.RedactedKeys {
		if attr, ok := resourceConfigAttr(resource.IntegrationType, key); ok {
			withheld[attr] = true
		}
	}

	var unmapped []string
	for key, value := range resource.Configuration {
		attr, ok := resourceConfigAttr(resource.IntegrationType, key)
		if !ok {
			unmapped = append(unmapped, key)
			continue
		}
		if withheld[attr] {
			continue
		}
		if err := setResourceConfigAttr(d, attr, value); err != nil {
			return diag.FromErr(err)
		}
	}

	if len(unmapped) > 0 {
		// Not an error: the stored blob carries keys the provider never wrote
		// (`version`, `name`, `type`), and an integration added after this table
		// will land here too. Logging keeps that visible without failing refresh.
		tflog.Debug(ctx, "resource config keys had no schema attribute", map[string]interface{}{
			"resource_id": d.Id(),
			"type":        resource.IntegrationType,
			"keys":        strings.Join(unmapped, ","),
		})
	}

	return nil
}

// setResourceConfigAttr writes one config value, normalising the shapes YAML
// decoding produces into what the schema expects.
func setResourceConfigAttr(d *schema.ResourceData, attr string, value interface{}) error {
	switch attr {
	case "hosts":
		// serverlist joins hosts with newlines on the way out; split them back
		// into the list the schema declares.
		switch t := value.(type) {
		case string:
			var hosts []string
			for _, h := range strings.Split(t, "\n") {
				if h = strings.TrimSpace(h); h != "" {
					hosts = append(hosts, h)
				}
			}
			return d.Set(attr, hosts)
		case []interface{}:
			return d.Set(attr, t)
		}
	case "port", "webui_port":
		// Ports round-trip as strings in the schema but decode as numbers.
		return d.Set(attr, fmt.Sprintf("%v", value))
	}

	return d.Set(attr, value)
}
