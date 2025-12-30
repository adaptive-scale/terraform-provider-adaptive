// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// providerFactories are used to instantiate a provider during acceptance testing.
// The factory function will be invoked for every Terraform CLI command executed
// to create a provider server to which the CLI can reattach.
var providerFactories = map[string]func() (*schema.Provider, error){
	"adaptive": func() (*schema.Provider, error) {
		return New("dev")(), nil
	},
}

func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestTryReadingServiceToken(t *testing.T) {
	tests := []struct {
		name          string
		inputToken    string
		inputURL      string
		expectedToken string
		expectedURL   string
		expectError   bool
	}{
		{
			name:          "empty token",
			inputToken:    "",
			inputURL:      "https://app.adaptive.live",
			expectedToken: "",
			expectedURL:   "",
			expectError:   true,
		},
		{
			name:          "plain token string",
			inputToken:    "plain-token-123",
			inputURL:      "https://app.adaptive.live",
			expectedToken: "plain-token-123",
			expectedURL:   "https://app.adaptive.live",
			expectError:   false,
		},
		{
			name: "simple JSON token",
			inputToken: `{
				"token": "json-token-456",
				"url": "https://custom.adaptive.live"
			}`,
			inputURL:      "https://app.adaptive.live",
			expectedToken: "json-token-456",
			expectedURL:   "https://custom.adaptive.live",
			expectError:   false,
		},
		{
			name: "deployments config with default",
			inputToken: `{
				"deployments": {
					"": {
						"url": "https://env01.staging.adaptive.live",
						"token": "V8lJjepkdhpFTuYFZj7HabVjT3s47yfralgpHExQhYgO8VAb6qxLabDHC0DbaAD5",
						"name": "",
						"default": true
					},
					"http://localhost:8080": {
						"url": "http://localhost:8080",
						"token": "g6qBH5lyVB7d0JJ3OKAyEU0ss9Gi8SFG5LJgDqSLVYXQlX1sUz6kiBVUWoi8j9uf",
						"name": "http://localhost:8080",
						"default": false
					}
				}
			}`,
			inputURL:      "https://app.adaptive.live",
			expectedToken: "V8lJjepkdhpFTuYFZj7HabVjT3s47yfralgpHExQhYgO8VAb6qxLabDHC0DbaAD5",
			expectedURL:   "https://env01.staging.adaptive.live",
			expectError:   false,
		},
		{
			name: "deployments config without default",
			inputToken: `{
				"deployments": {
					"prod": {
						"url": "https://prod.adaptive.live",
						"token": "prod-token-789",
						"name": "production",
						"default": false
					},
					"dev": {
						"url": "https://dev.adaptive.live",
						"token": "dev-token-101",
						"name": "development",
						"default": false
					}
				}
			}`,
			inputURL:      "https://app.adaptive.live",
			expectedToken: "prod-token-789", // Should return first deployment
			expectedURL:   "https://prod.adaptive.live",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, url, err := tryReadingServiceToken(tt.inputToken, tt.inputURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if token != tt.expectedToken {
				t.Errorf("expected token %q, got %q", tt.expectedToken, token)
			}

			if url != tt.expectedURL {
				t.Errorf("expected URL %q, got %q", tt.expectedURL, url)
			}
		})
	}
}

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
}
