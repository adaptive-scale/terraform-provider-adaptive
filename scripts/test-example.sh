#!/bin/bash

# Script to test a specific Terraform example
# Usage: ./scripts/test-example.sh <example-name>

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <example-name>"
    echo "Available examples:"
    ls examples/ | grep -v README.md
    exit 1
fi

EXAMPLE_NAME="$1"
EXAMPLE_DIR="examples/${EXAMPLE_NAME}"

if [ ! -d "$EXAMPLE_DIR" ]; then
    echo "Error: Example '${EXAMPLE_NAME}' does not exist in examples/ directory"
    exit 1
fi

echo "Testing example: ${EXAMPLE_NAME}"
cd "$EXAMPLE_DIR"

# Remove lock file to avoid checksum issues with rebuilt provider
if [ -f ".terraform.lock.hcl" ]; then
    rm .terraform.lock.hcl
fi

echo "Initializing Terraform..."
terraform init -upgrade

echo "Validating configuration..."
terraform validate

echo "Planning deployment..."
terraform plan

echo "✅ Example '${EXAMPLE_NAME}' passed all tests!"