#!/bin/bash

# Script to build and install the Terraform provider locally
# Usage: ./scripts/build-install.sh

set -e

echo "Building Terraform provider..."
go build -o terraform-provider-adaptive

echo "Installing provider to local plugins directory..."
VERSION="0.1.6"
OS_ARCH="darwin_arm64"
PLUGIN_DIR="$HOME/.terraform.d/plugins/adaptive-scale/local/adaptive/${VERSION}/${OS_ARCH}"

mkdir -p "$PLUGIN_DIR"
mv terraform-provider-adaptive "$PLUGIN_DIR/"

echo "Provider installed successfully to $PLUGIN_DIR"
echo "You can now use version ${VERSION} in your Terraform configurations"