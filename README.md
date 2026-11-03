# Terraform Provider for Adaptive

[![Terraform Registry](https://img.shields.io/badge/terraform-adaptive_scale-rgb(31%2C%2020%2C%20139))](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/)
[![License](https://img.shields.io/badge/license-MPL--2.0-blue.svg)](LICENSE)

The [Adaptive Terraform Provider](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/) enables you to manage your [Adaptive](https://adaptive.live) infrastructure using Infrastructure as Code (IaC).

## Features

- **80+ Integrations** - Connect to databases, cloud platforms, Kubernetes, identity providers, and more
- **Fine-grained Access Control** - Define authorizations with specific permissions per resource type
- **Just-In-Time Access** - Configure JIT access with approval workflows, multi-approver rules, and scheduled auto-approval
- **Data Masking** - Mask columns of a database resource so session users never see the raw values
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
| **Databases** | PostgreSQL, MySQL, MongoDB, CockroachDB, ClickHouse, Snowflake, SQL Server, YugabyteDB, Oracle, Redis, ProxySQL, BigQuery, Elasticsearch |
| **Cloud** | AWS (including ElastiCache, DocumentDB, CloudWatch Logs, Secrets Manager), Azure, GCP, DigitalOcean, Heroku |
| **Container** | Kubernetes |
| **Identity** | Okta, OneLogin, JumpCloud, Google, LDAP, 1Password |
| **Monitoring** | Datadog, Splunk, Coralogix |
| **Network** | SSH, ZeroTier, Cisco/Fortinet/Palo Alto NGFW, Juniper, Sophos, secure tunnels |
| **Remote Desktop** | RDP (single host, fleet, or LDAP-authenticated), VNC, Windows server groups, Chrome |
| **Messaging** | RabbitMQ, Kafka, Confluent, Microsoft Teams, Slack |

See the [examples directory](examples/) for configuration examples of each integration.

## Resources

The provider includes 7 resource types:

| Resource | Description |
|----------|-------------|
| `adaptive_resource` | Connection to external services (databases, cloud platforms, etc.) |
| `adaptive_endpoint` | Secure access point with TTL, JIT, and user assignments |
| `adaptive_authorization` | Permission policy for fine-grained access control |
| `adaptive_group` | User and endpoint organization for access management |
| `adaptive_script` | Command execution on endpoints |
| `adaptive_schedule` | Auto-approval or auto-rejection of access requests in a recurring window |
| `adaptive_dataprotection` | Column-level masking policy for a database resource |

## Refresh, Drift and Import

Every resource now reads its real state back from the platform, so `terraform plan` reports changes made outside Terraform and a resource deleted out-of-band is planned for recreation instead of failing the refresh. All seven support `terraform import` by ID.

Refresh is conservative about optional arguments: one your configuration never set is generally not populated from a server-side default, since that would show up as a diff on every plan. Resource credentials and script bodies depend on backend redaction; backend fingerprints detect supported out-of-band changes without returning the withheld values. `adaptive_resource.applied_digests` retains the recorded baseline across refreshes. Import cannot recover withheld credentials, and resource updates now reject omission of known held credentials before replacing configuration; explicitly setting a supported optional string credential to `""` authorizes clearing it. Missing fingerprint metadata is not evidence that a credential is absent. Credential arguments marked sensitive still persist in Terraform state: protect state and saved plans. See the resource's Secret Arguments section for coverage and compatibility limits.

### Migrating from `adaptive_msteams_workflow`

The standalone `adaptive_msteams_workflow` resource has been removed. Microsoft Teams workflow webhooks are now an `adaptive_resource` type, alongside every other integration:

```hcl
resource "adaptive_resource" "teams_alerts" {
  name        = "teams-alerts"
  type        = "msteams_workflow"
  webhook_url = var.teams_webhook_url
}
```

To migrate without recreating the integration, remove the old resource from state and import it under the new address:

```bash
terraform state rm adaptive_msteams_workflow.example
terraform import adaptive_resource.teams_alerts <resource-id>
```

`webhook_url` is sensitive and is not returned by the API, so set it in your configuration before applying.

## Documentation

- [Terraform Guide](https://documentation.adaptive.live/developer-guide/terraform)
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

The ordinary offline suite is `make test`; `make build` compiles the provider. To exercise actual Terraform plan/apply/import behavior against a loopback fake API (no Adaptive account or live infrastructure), install Terraform and run:

```bash
ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^TestSecretTerraformEngine$' -v -count=1 -timeout=180s
```

The opt-in test builds the local plugin, isolates CLI configuration and credentials, and verifies repeated secret-drift refreshes, saved-plan reconciliation, import update protection, and explicit clearing. It uses the existing sensitive-state credential path, not native Terraform write-only arguments. The dedicated offline CLI CI job runs this test with Terraform 1.14.3; without the opt-in flag the engine test is skipped.

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
