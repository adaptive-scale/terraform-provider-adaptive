#!/bin/bash

# Script to generate Terraform provider documentation
# Usage: ./scripts/generate-docs.sh

set -e

echo "Generating Terraform provider documentation..."

# Generate documentation using tfplugindocs
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

echo "✅ Documentation generated successfully!"
echo "Generated files:"
echo "  - docs/index.md (provider documentation)"
echo "  - docs/resources/ (resource documentation)"
echo ""
echo "Files updated in docs/ directory"