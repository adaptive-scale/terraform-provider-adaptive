// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	client "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"service_token": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Service account token for authenticating with the Adaptive service.",
					DefaultFunc: schema.EnvDefaultFunc("ADAPTIVE_SVC_TOKEN", nil),
				},
				"workspace_url": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "The workspace to use for the provider. If not set, the default workspace will be used app.adaptive.live",
				},
			},
			// DataSourcesMap: map[string]*schema.Resource{
			// 	"adaptive_data_source": dataSourceScaffolding(),
			// },
			ResourcesMap: map[string]*schema.Resource{
				"adaptive_gcp":         resourceAdaptiveGCP(),
				"adaptive_aws":         resourceAdaptiveAWS(),
				"adaptive_azure":       resourceAdaptiveAzure(),
				"adaptive_google":      resourceAdaptiveGoogle(),
				"adaptive_okta":        resourceAdaptiveOkta(),
				"adaptive_ssh":         resourceAdaptiveSSH(),
				"adaptive_servicelist": resourceAdaptiveServiceList(),
				"adaptive_mysql":       resourceAdaptiveMySQL(),
				"adaptive_mongodb":     resourceAdaptiveMongo(),
				"adaptive_postgres":    resourceAdaptivePostgres(),
				"adaptive_cockroachdb": resourceAdaptiveCockroachDB(),
				"adaptive_session":     resourceAdaptiveSession(),
				"adaptive_users":       users(),
			},
			ConfigureContextFunc: providerConfigure,
		}
		return p
	}
}
func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	serviceToken := d.Get("service_token").(string)
	workspaceURL := d.Get("workspace_url").(string)

	if serviceToken == "" {
		return nil, diag.Errorf("The 'serviceToken' field is required.")
	}

	// Initialize the Adaptive API client with the provided service token.
	c := client.NewClient(serviceToken, workspaceURL)

	return c, nil
}
