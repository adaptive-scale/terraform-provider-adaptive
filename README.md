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

- [Terraform](https://www.terraform.io/downloads.html) >= 1.11 (CLI contract tests pin 1.11.4 and 1.14.3)
- An authorized Adaptive service token and the intended workspace URL. The Adaptive CLI is optional when explicit credentials are provided.

### 2. Authenticate

Obtain a least-privilege service token through your approved Adaptive administration workflow. Select the target workspace explicitly; do not place real tokens in checked-in files.

```bash
export ADAPTIVE_SVC_TOKEN="your-service-token"
export ADAPTIVE_URL="https://app.adaptive.live"
```

### 3. Configure the Provider

```hcl
terraform {
  required_version = ">= 1.11"
  required_providers {
    adaptive = {
      source  = "adaptive-scale/adaptive"
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
  ttl      = "6h"
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

See the [resource documentation](docs/resources/resource.md) for integration-specific inputs and configuration examples.

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

Optional-field ownership is contract-specific: omitted endpoint policy and selected integration controls retain observed values, while full-replacement fields can reset on removal. See each resource document for explicit omission and clearing rules. Resource credentials and script bodies depend on backend redaction; backend fingerprints detect supported out-of-band changes without returning the withheld values. `adaptive_resource.applied_digests` retains the recorded baseline across refreshes. Import cannot recover withheld credentials, and resource updates now reject omission of known held credentials before replacing configuration; explicitly setting a supported optional string credential to `""` authorizes clearing it. Missing fingerprint metadata is not evidence that a credential is absent. Credential arguments marked sensitive still persist in Terraform state: protect state and saved plans. See the resource's Secret Arguments section for coverage and compatibility limits.

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
- [Provider Configuration](docs/index.md)
- [Integration Contracts](docs/resources/resource.md)
- [Endpoint Policy](docs/resources/endpoint.md)
- [Authorization Policies](docs/resources/authorization.md)
- [Provider Documentation](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest/docs)
- [Parity Delivery and Compatibility](docs/parity-remediation-plan.md)

## Examples

Configuration examples are embedded in the generated [resource documents](docs/resources/resource.md). This checkout does not ship an `examples/` directory; optional example-generation scripts are development scaffolding, not preverified live targets.

The parity-hardening stack is unreleased. Use the locally built plugin for testing; these local commits do not publish a provider. For production, select and pin an actually published provider version compatible with the deployed backend.

## Development

For contributors and local development:

The ordinary offline suite is `make test`; `make build` compiles the provider. To exercise actual Terraform plan/apply/import behavior against a loopback fake API (no Adaptive account or live infrastructure), install Terraform and run:

```bash
ADAPTIVE_ENGINE_TEST=1 go test ./internal/provider -run '^Test(Secret|Mappings|Endpoint|Cloud|IntegrationFields|Families|CoreLifecycle)TerraformEngine$' -v -count=1 -timeout=180s
```

The seven opt-in suites build the local plugin and isolate CLI configuration/credentials against loopback fake APIs. Together they exercise all seven resource families plus mapping, policy and credential lifecycle regressions. CI runs the actual suites with Terraform 1.11.4 and 1.14.3; without `ADAPTIVE_ENGINE_TEST=1` they are skipped. They use legacy sensitive-state credentials, not native write-only inputs. Configuration persistence is not live IAM, TLS, connection, policy-enforcement or Kubernetes-cleanup proof.

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
