#!/bin/bash

# Script to test all Terraform examples
# Usage: ./scripts/test-all-examples.sh

set -e

echo "Testing all examples..."

FAILED_EXAMPLES=()
PASSED_EXAMPLES=()

for example_dir in examples/*/; do
    if [ -d "$example_dir" ] && [ "$(basename "$example_dir")" != "README.md" ]; then
        EXAMPLE_NAME=$(basename "$example_dir")
        echo ""
        echo "========================================="
        echo "Testing example: ${EXAMPLE_NAME}"
        echo "========================================="

        cd "$example_dir"

        if ./../../scripts/test-example.sh "$EXAMPLE_NAME" 2>/dev/null; then
            echo "✅ ${EXAMPLE_NAME} - PASSED"
            PASSED_EXAMPLES+=("$EXAMPLE_NAME")
        else
            echo "❌ ${EXAMPLE_NAME} - FAILED"
            FAILED_EXAMPLES+=("$EXAMPLE_NAME")
        fi

        # Go back to project root
        cd ../..
    fi
done

echo ""
echo "========================================="
echo "SUMMARY"
echo "========================================="
echo "Passed: ${#PASSED_EXAMPLES[@]}"
echo "Failed: ${#FAILED_EXAMPLES[@]}"

if [ ${#PASSED_EXAMPLES[@]} -gt 0 ]; then
    echo ""
    echo "✅ PASSED EXAMPLES:"
    for example in "${PASSED_EXAMPLES[@]}"; do
        echo "  - $example"
    done
fi

if [ ${#FAILED_EXAMPLES[@]} -gt 0 ]; then
    echo ""
    echo "❌ FAILED EXAMPLES:"
    for example in "${FAILED_EXAMPLES[@]}"; do
        echo "  - $example"
    done
    exit 1
else
    echo ""
    echo "🎉 All examples passed!"
fi