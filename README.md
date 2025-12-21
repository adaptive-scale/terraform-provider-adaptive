# Terraform Provider for Adaptive

[![https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/](<https://img.shields.io/badge/terraform-adaptive_scale-rgb(31%2C%2020%2C%20139)>)](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/)

[Adaptive's terraform provider](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/) can used to configure and manage your resource on Adaptive.

## Authentication

If you do not already have it installed, download and install [adaptive-cli](https://docs.adaptive.live/cli), to login and generate a service-token.

## Environment Variables

Env vars can be used to provide your credentials to the provider

```bash
$ export ADAPTIVE_SVC_TOKEN="<service_token>"
```

Provider initialization

```hcl
provider "adaptive" {}
```

## Development

This project includes several shell scripts to automate common development workflows. See the [scripts/README.md](scripts/README.md) for detailed information.

### Quick Start for Developers

```bash
# Build and install the provider locally
./scripts/build-install.sh

# Test all examples
./scripts/test-all-examples.sh

# Or test a specific example
./scripts/test-example.sh kubernetes

# Run the full development workflow
./scripts/dev-workflow.sh
```

## Useful Links

- [Provider documentation](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/docs)
- [Examples](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/docs/guides/adding_a_resource)

## Contributions

If you have something to contribute, feature requests, find a bug or feedback, please reachout to use at [support@adaptive.live](mailto:support@adaptive.live)
