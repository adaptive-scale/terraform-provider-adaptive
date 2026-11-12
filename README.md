# Terraform Provider for Adaptive

[![Terraform Registry](https://img.shields.io/badge/terraform-adaptive_scale-rgb(31%2C%2020%2C%20139))](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/)
[![License](https://img.shields.io/badge/license-MPL--2.0-blue.svg)](LICENSE)

The [Adaptive Terraform Provider](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/) enables you to manage your [Adaptive](https://adaptive.live) infrastructure using Infrastructure as Code (IaC).

## Features

- **50+ Integrations** - Connect to databases, cloud platforms, Kubernetes, identity providers, and more
- **Fine-grained Access Control** - Define authorizations with specific permissions per resource type
- **Just-In-Time Access** - Configure JIT access with approval workflows
- **Group Management** - Organize users and endpoints for simplified access management

## Quick Start

### 1. Install Prerequisites

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Adaptive CLI](https://docs.adaptive.live/cli)

### 2. Authenticate

```bash
# Login to Adaptive
adaptive login

# Generate a service token
adaptive service-token create --name terraform-provider

# Set the token as an environment variable
export ADAPTIVE_SVC_TOKEN="your-service-token"
```

### 3. Configure the Provider

```hcl
terraform {
  required_providers {
    adaptive = {
      source  = "adaptive-scale/adaptive"
      version = "~> 1.0"
    }
  }
}

provider "adaptive" {}
```

### 4. Create Resources

```hcl
# Create a PostgreSQL resource
resource "adaptive_resource" "postgres" {
  name          = "my-database"
  type          = "postgres"
  host          = "postgres.example.com"
  port          = "5432"
  username      = "admin"
  password      = var.db_password
  database_name = "myapp"
}

# Create an endpoint for access
resource "adaptive_endpoint" "postgres_access" {
  name     = "postgres-access"
  resource = adaptive_resource.postgres.name
  ttl      = "8h"
  users    = ["developer@example.com"]
}
```

### 5. Apply Configuration

```bash
terraform init
terraform plan
terraform apply
```

## Supported Integrations

| Category | Integrations |
|----------|-------------|
| **Databases** | PostgreSQL, MySQL, MongoDB, CockroachDB, ClickHouse, Snowflake, SQL Server, YugabyteDB, Elasticsearch |
| **Cloud** | AWS, Azure, GCP |
| **Container** | Kubernetes |
| **Identity** | Okta, OneLogin, JumpCloud, Google |
| **Monitoring** | Datadog, Splunk, Coralogix |
| **Network** | SSH, ZeroTier, Cisco/Fortinet/Palo Alto NGFW |
| **Messaging** | RabbitMQ, Microsoft Teams |

See the [examples directory](examples/) for configuration examples of each integration.

## Resources

The provider includes 8 resource types:

| Resource | Description |
|----------|-------------|
| `adaptive_resource` | Connection to external services (databases, cloud platforms, etc.) |
| `adaptive_endpoint` | Secure access point with TTL, JIT, and user assignments |
| `adaptive_authorization` | Permission policy for fine-grained access control |
| `adaptive_group` | User and endpoint organization for access management |
| `adaptive_script` | Command execution on endpoints |
| `adaptive_schedule` | Scheduled access approval or rejection |
| `adaptive_data_protection` | Database masking policies |
| `adaptive_msteams_workflow` | Microsoft Teams workflow integration |

## Server compatibility

The provider supports the Terraform API on Adaptive `release/v0.9.x`, including
endpoint JIT policies, script metadata, full schedule refresh, and data protection.
Existing resource names, attributes, defaults, list types, and state versions are
preserved; existing configurations do not require a state migration.

Refresh accepts both legacy PascalCase and current camelCase responses, including
HTTP 200 and 202. Older responses leave unreported fields unchanged. A server
without a read route produces a warning and retains state; an object reported as
missing by the API is removed from state so Terraform can recreate it. Permission
and server errors stop refresh without dropping the object.

Credentials, composite settings containing redacted credentials, and omitted
script bodies retain their configured values. Where available, `redacted_digests`
and `command_digest` record opaque server fingerprints and warn about subsequent
external changes. They do not recover plaintext or automatically restore secrets.
Resource refresh updates mapped public configuration fields already represented
in state; omitted and unmapped settings remain unchanged. Server-selected endpoint
defaults remain inherited when the corresponding configuration is empty.

New endpoint policy and script metadata fields are optional. Omission retains the
server value after refresh; explicit `false`, `0`, and supported empty strings are
sent as configured. New features require server support. Older servers may ignore
fields they do not recognize; the provider does not emulate those features.

Destroying `adaptive_data_protection` disables masking through the server API. It
retains the server's policy record, generated authorization, and existing views.

## Documentation

- [Getting Started Guide](docs/guides/getting-started.md)
- [Integration Guide](docs/guides/integrations.md)
- [Access Control Guide](docs/guides/access-control.md)
- [Provider Documentation](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/docs)
- [Example Configurations](examples/)

## Examples

The [examples](examples/) directory contains working configurations for all supported integrations:

```bash
# Navigate to an example
cd examples/postgres

# Initialize and apply
terraform init
terraform plan
```

## Development

For contributors and local development:

```bash
# Build and install locally
./scripts/build-install.sh

# Test examples
./scripts/test-all-examples.sh
./scripts/test-example.sh kubernetes

# Generate documentation
./scripts/generate-docs.sh

# Full development workflow
./scripts/dev-workflow.sh
```

See [scripts/README.md](scripts/README.md) for detailed development documentation.

## Support

- **Documentation**: [docs.adaptive.live](https://docs.adaptive.live)
- **Issues**: [GitHub Issues](https://github.com/adaptive-scale/terraform-provider-adaptive/issues)
- **Email**: [support@adaptive.live](mailto:support@adaptive.live)

## License

This provider is distributed under the [Mozilla Public License 2.0](LICENSE).
