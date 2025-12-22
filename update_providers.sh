#!/bin/bash

# Update all provider.tf files to use the correct adaptive provider configuration

find examples -name "provider.tf" | while read -r file; do
  echo "Updating $file"
  
  # Create the new content
  cat > "$file" << 'EOL'
terraform {
  required_providers {
    adaptive = {
      source  = "adaptive-scale/local/adaptive"
      version = "0.1.6"
    }
  }
}

provider "adaptive" {}
EOL
done

echo "All provider.tf files updated"
