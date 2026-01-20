// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/components"
	client "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
					Optional:    true,
					Description: "Service account token for authenticating with the Adaptive service. If not provided, provider will default to reading token from default adaptive-cli",
					DefaultFunc: schema.EnvDefaultFunc("ADAPTIVE_SVC_TOKEN", ""),
				},
				"workspace_url": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("ADAPTIVE_URL", "https://app.adaptive.live"),
					Description: "The workspace to use for the provider. If not set, the default workspace will be used app.adaptive.live",
				},
			},

			ResourcesMap: map[string]*schema.Resource{
				"adaptive_endpoint":      components.ResourceAdaptiveSession(),
				"adaptive_resource":      components.ResourceAdaptiveResource(),
				"adaptive_authorization": components.ResourceAdaptiveAuthorization(),
				"adaptive_group":         components.ResourceAdaptiveTeam(),
				"adaptive_script":        components.ResourceAdaptiveScript(),
			}, ConfigureContextFunc: providerConfigure,
		}
		return p
	}
}

type AdaptiveCLISVCToken struct {
	Token        string `json:"token"`
	WorkspaceURL string `json:"url"`
}

type AdaptiveDeploymentConfig struct {
	URL     string `json:"url"`
	Token   string `json:"token"`
	Name    string `json:"name,omitempty"`
	Default bool   `json:"default,omitempty"`
}

type AdaptiveDeploymentsConfig struct {
	Deployments map[string]AdaptiveDeploymentConfig `json:"deployments"`
}

func tryReadingServiceToken(potentialToken, workspaceURL string) (string, string, error) {
	if potentialToken == "" {
		return "", "", errors.New("'serviceToken' field cannot be empty")
	}

	// First, try to parse as deployments config
	var deploymentsConfig AdaptiveDeploymentsConfig
	if err := json.Unmarshal([]byte(potentialToken), &deploymentsConfig); err == nil {
		// Find the default deployment
		for _, deployment := range deploymentsConfig.Deployments {
			if deployment.Default {
				return deployment.Token, deployment.URL, nil
			}
		}
		// If no default found, use the first deployment
		for _, deployment := range deploymentsConfig.Deployments {
			return deployment.Token, deployment.URL, nil
		}
	}

	// Fallback: try to parse as simple token config
	var _token AdaptiveCLISVCToken
	if _err := json.Unmarshal([]byte(potentialToken), &_token); _err == nil {
		return _token.Token, _token.WorkspaceURL, nil
	}

	// Final fallback: treat as plain token string
	return potentialToken, workspaceURL, nil
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	serviceToken := d.Get("service_token").(string)
	workspaceURL := d.Get("workspace_url").(string)
	if serviceToken == "" {
		tflog.Debug(ctx, "empty token initialization, defaulting to adaptive-cli config folder")

		defaultLocation := "~/.adaptive/token"
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, diag.FromErr(fmt.Errorf("service_token not provided and failed to read token from default location (%s). reason: %w", defaultLocation, err))
		}
		serviceTokenJSON, err := ioutil.ReadFile(path.Join(homeDir, ".adaptive", "token"))
		if err != nil {
			return nil, diag.FromErr(fmt.Errorf("service_token not provided and failed to read token from default location (%s). reason: %w", defaultLocation, err))
		}
		// let tryReadingServiceToken parse the json
		serviceToken = string(serviceTokenJSON)
	}

	svcToken, wsURL, err := tryReadingServiceToken(serviceToken, workspaceURL)
	if err != nil {
		return nil, diag.Errorf(fmt.Sprintf("bad service token: %s", err))
	}
	c := client.NewClient(svcToken, wsURL)

	return c, nil
}
