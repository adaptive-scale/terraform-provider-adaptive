package components

import (
	"strings"
	"testing"
)

// Guard the documented runtime boundary, not backend enforcement: these tests
// only inspect the Terraform schema description.
func TestAWSAllowedCommandsDescriptionDisclosesShellFallback(t *testing.T) {
	description := ResourceAdaptiveResource().Schema["enable_allowed_commands"].Description
	for _, disclosure := range []string{
		"both command and parameter are nonempty strings",
		"service/subcommand",
		"CLI parameters are absent or either value is missing, empty, or not a string",
		"interactive /bin/bash shell before allowlist validation",
		"not a command-only security boundary",
		"Empty or whitespace-only allowed_commands means allow all",
	} {
		if !strings.Contains(description, disclosure) {
			t.Errorf("AWS allowlist description must disclose %q", disclosure)
		}
	}
	if strings.Contains(description, "instead of an interactive shell") {
		t.Error("AWS allowlist description must not claim interactive shells are disabled")
	}
}
