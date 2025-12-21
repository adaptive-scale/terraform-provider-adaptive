#!/bin/bash

# Script to clean build artifacts and caches
# Usage: ./scripts/clean.sh

set -e

echo "Cleaning build artifacts..."

# Remove built binary
if [ -f "terraform-provider-adaptive" ]; then
    rm terraform-provider-adaptive
    echo "Removed terraform-provider-adaptive binary"
fi

# Remove bin directory
if [ -d "bin" ]; then
    rm -rf bin/
    echo "Removed bin/ directory"
fi

# Clean Terraform caches in examples
echo "Cleaning Terraform caches in examples..."
for example_dir in examples/*/; do
    if [ -d "$example_dir" ]; then
        cd "$example_dir"
        if [ -d ".terraform" ]; then
            rm -rf .terraform/
            echo "Cleaned .terraform in $(basename "$example_dir")"
        fi
        if [ -f ".terraform.lock.hcl" ]; then
            rm .terraform.lock.hcl
            echo "Removed .terraform.lock.hcl in $(basename "$example_dir")"
        fi
        cd ../..
    fi
done

# Clean Go build cache
echo "Cleaning Go build cache..."
go clean -cache
go clean -testcache
go clean -modcache

echo "✅ Cleanup completed!"