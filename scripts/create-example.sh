#!/bin/bash

# Script to create a new Terraform example
# Usage: ./scripts/create-example.sh <integration-type>

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <integration-type>"
    echo "Example: $0 kubernetes"
    exit 1
fi

INTEGRATION_TYPE="$1"
EXAMPLE_DIR="examples/${INTEGRATION_TYPE}"

if [ -d "$EXAMPLE_DIR" ]; then
    echo "Error: Example '${INTEGRATION_TYPE}' already exists"
    exit 1
fi

echo "Creating example for integration: ${INTEGRATION_TYPE}"

# Create directory
mkdir -p "$EXAMPLE_DIR"

# Copy provider.tf from an existing example (using aws as template)
if [ -f "examples/aws/provider.tf" ]; then
    cp "examples/aws/provider.tf" "$EXAMPLE_DIR/"
else
    # Create a basic provider.tf if aws example doesn't exist
    cat > "$EXAMPLE_DIR/provider.tf" << 'EOF'
terraform {
  required_providers {
    adaptive = {
      source  = "adaptive-scale/local/adaptive"
      version = "0.1.6"
    }
  }
}

provider "adaptive" {}
EOF
fi

# Create main.tf with basic structure
cat > "$EXAMPLE_DIR/main.tf" << EOF
resource "adaptive_resource" "${INTEGRATION_TYPE}" {
  type = "${INTEGRATION_TYPE}"

  name = "${INTEGRATION_TYPE}-test"
  # Add integration-specific configuration here
}
EOF

echo "✅ Example '${INTEGRATION_TYPE}' created successfully!"
echo "Files created:"
echo "  - $EXAMPLE_DIR/provider.tf"
echo "  - $EXAMPLE_DIR/main.tf"
echo ""
echo "Next steps:"
echo "1. Edit $EXAMPLE_DIR/main.tf to add integration-specific configuration"
echo "2. Test with: ./scripts/test-example.sh ${INTEGRATION_TYPE}"