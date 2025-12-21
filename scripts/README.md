# Development Scripts

This directory contains shell scripts to automate common development workflows for the Terraform Adaptive provider.

## Available Scripts

### `build-install.sh`
Builds the Terraform provider and installs it to the local plugins directory.
```bash
./scripts/build-install.sh
```

### `test-example.sh <example-name>`
Tests a specific example by running `terraform init`, `validate`, and `plan`.
```bash
./scripts/test-example.sh kubernetes
```

### `test-all-examples.sh`
Tests all examples in the `examples/` directory.
```bash
./scripts/test-all-examples.sh
```

### `create-example.sh <integration-type>`
Creates a new example directory with basic `provider.tf` and `main.tf` files.
```bash
./scripts/create-example.sh new-integration
```

### `clean.sh`
Cleans build artifacts, Terraform caches, and Go build cache.
```bash
./scripts/clean.sh
```

### `dev-workflow.sh [example-name]`
Runs the full development workflow: build, install, and test.
If an example name is provided, tests only that example; otherwise tests all examples.
```bash
./scripts/dev-workflow.sh kubernetes  # Test specific example
./scripts/dev-workflow.sh             # Test all examples
```

## Usage Examples

### Development Cycle
```bash
# Make changes to the provider code
# Then run the full workflow
./scripts/dev-workflow.sh

# Or test just the example you're working on
./scripts/dev-workflow.sh kubernetes
```

### Adding a New Integration
```bash
# Create the example structure
./scripts/create-example.sh my-new-integration

# Edit the main.tf file with integration-specific configuration
# Then test it
./scripts/test-example.sh my-new-integration
```

### Cleanup
```bash
# Clean everything before a fresh build
./scripts/clean.sh
./scripts/build-install.sh
```