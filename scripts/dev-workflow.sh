#!/bin/bash

# Script to run the full development workflow
# Usage: ./scripts/dev-workflow.sh [example-name]

set -e

echo "🚀 Starting development workflow..."

# Build and install provider
echo ""
echo "1. Building and installing provider..."
./scripts/build-install.sh

# Test specific example or all examples
if [ $# -eq 1 ]; then
    EXAMPLE_NAME="$1"
    echo ""
    echo "2. Testing example: ${EXAMPLE_NAME}"
    if [ -d "examples/${EXAMPLE_NAME}" ]; then
        ./scripts/test-example.sh "$EXAMPLE_NAME"
    else
        echo "❌ Example '${EXAMPLE_NAME}' does not exist"
        exit 1
    fi
else
    echo ""
    echo "2. Testing all examples..."
    ./scripts/test-all-examples.sh
fi

echo ""
echo "✅ Development workflow completed successfully!"