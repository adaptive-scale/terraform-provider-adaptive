// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccAdaptiveDLPPolicy_basic exercises the full CRUD + import lifecycle of
// adaptive_dlp_policy against a real Adaptive workspace. It only runs when
// TF_ACC=1 (standard acceptance-test gate) and ADAPTIVE_SVC_TOKEN is set.
//
//	TF_ACC=1 ADAPTIVE_SVC_TOKEN=... ADAPTIVE_URL=... go test ./internal/provider -run TestAccAdaptiveDLPPolicy -v
func TestAccAdaptiveDLPPolicy_basic(t *testing.T) {
	if os.Getenv("ADAPTIVE_SVC_TOKEN") == "" {
		t.Skip("ADAPTIVE_SVC_TOKEN not set, skipping acceptance test")
	}

	name := fmt.Sprintf("tf-acc-dlp-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdaptiveDLPPolicyConfig(name, 500, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "name", name),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "enabled", "true"),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "block_screenshot", "true"),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "max_export_rows", "500"),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "blocked_surfaces.0", "webshell"),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "applies_to_labels.0", "pii"),
					resource.TestCheckResourceAttrSet("adaptive_dlp_policy.test", "policy_id"),
				),
			},
			{
				// In-place update: the backend upserts by name.
				Config: testAccAdaptiveDLPPolicyConfig(name, 100, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "max_export_rows", "100"),
					resource.TestCheckResourceAttr("adaptive_dlp_policy.test", "block_screenshot", "false"),
				),
			},
			{
				// Import by name (the resource ID is the policy name).
				ResourceName:      "adaptive_dlp_policy.test",
				ImportState:       true,
				ImportStateId:     name,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAdaptiveDLPPolicyConfig(name string, maxExportRows int, blockScreenshot bool) string {
	return fmt.Sprintf(`
resource "adaptive_dlp_policy" "test" {
  name                     = %q
  description              = "acceptance test policy"
  enabled                  = true
  block_screenshot         = %t
  block_clipboard_copy     = true
  block_clipboard_paste    = false
  block_file_transfer      = true
  block_bulk_export        = true
  max_export_rows          = %d
  block_session_log_export = true
  blocked_surfaces         = ["webshell"]
  exceptions               = ["\"sre\" in teams && deviceTrusted"]
  applies_to_labels        = ["pii"]
  workspace_default        = false
}
`, name, blockScreenshot, maxExportRows)
}
