---
page_title: "Adaptive Provider"
subcategory: ""
description: |-
  The Adaptive Terraform provider enables you to manage Adaptive resources, endpoints, groups, and authorizations as code.
---

# Adaptive Provider

The Adaptive Terraform provider allows you to manage your [Adaptive](https://adaptive.live) infrastructure using Infrastructure as Code (IaC). With this provider, you can:

- **Create and manage resources** - Connect databases, cloud platforms, Kubernetes clusters, and 82 registered integration types
- **Configure endpoints** - Set up secure access points with Just-In-Time (JIT) access controls
- **Define authorizations** - Create fine-grained permission policies for different resource types
- **Manage groups** - Organize users and endpoints into logical groups for access control
- **Manage scripts** - Store reusable command definitions for later invocation

## Getting Started

### Prerequisites

1. An Adaptive account - [Sign up](https://adaptive.live) if you don't have one
2. Terraform >= 1.11; CLI contract tests pin 1.11.4 and 1.14.3
3. An authorized service token and the intended workspace URL. The Adaptive CLI is optional with explicit credentials.

### Service Token

Obtain a least-privilege token through your approved Adaptive administration workflow. Keep it out of checked-in files. Explicit credentials avoid relying on a workstation's default CLI deployment.

### Provider Configuration

The provider can be configured using environment variables (recommended) or directly in the configuration.

#### Using Environment Variables (Recommended)

```bash
export ADAPTIVE_SVC_TOKEN="your-service-token"
export ADAPTIVE_URL="https://app.adaptive.live"
```

```terraform
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

#### Explicit Configuration

```terraform
provider "adaptive" {
  service_token = "your-service-token"
  workspace_url = "https://app.adaptive.live"  # Optional, defaults to app.adaptive.live
}
```

## Example Usage

Here's a complete example that creates a PostgreSQL resource, an endpoint, and assigns it to a group:

```terraform
# Create a PostgreSQL resource
resource "adaptive_resource" "postgres_db" {
  name     = "production-postgres"
  type     = "postgres"
  host     = "postgres.example.com"
  port     = "5432"
  username = "admin"
  password = var.db_password
  database_name = "myapp"
  ssl_mode = "require"
  tags     = ["production", "database"]
}

# Create an endpoint for the resource
resource "adaptive_endpoint" "postgres_endpoint" {
  name     = "postgres-access"
  resource = adaptive_resource.postgres_db.name
  ttl      = "6h"
  users    = ["developer@example.com", "dba@example.com"]
  tags     = ["production"]
}

# Create a group and assign the endpoint
resource "adaptive_group" "db_admins" {
  name      = "database-admins"
  members   = ["dba@example.com", "lead@example.com"]
  endpoints = [adaptive_endpoint.postgres_endpoint.name]
}
```

## Resources

- [adaptive_resource](resources/resource.md) - Manage connections to databases, cloud platforms, and other services
- [adaptive_endpoint](resources/endpoint.md) - Create secure access points to resources
- [adaptive_authorization](resources/authorization.md) - Define permission policies
- [adaptive_group](resources/group.md) - Organize users and endpoints
- [adaptive_script](resources/script.md) - Execute commands on endpoints
- [adaptive_schedule](resources/schedule.md) - Auto-approve or auto-reject access during recurring windows
- [adaptive_dataprotection](resources/dataprotection.md) - Mask columns of a database resource for session users

## Authentication Methods

The provider supports multiple authentication methods in order of precedence:

1. **Provider configuration** - `service_token` argument
2. **Environment variable** - `ADAPTIVE_SVC_TOKEN`
3. **Adaptive CLI** - Uses token from `adaptive login` if available

When the CLI config holds several deployments, the provider uses the one marked `"default": true`. If none is marked and there is more than one, it fails rather than guessing which workspace to write to — mark a default, or set `ADAPTIVE_SVC_TOKEN` and `ADAPTIVE_URL`.

## Supported Integrations

The Adaptive provider supports 80+ integration types including:

| Category | Integrations |
|----------|-------------|
| **Databases** | PostgreSQL, MySQL, MongoDB, CockroachDB, ClickHouse, Snowflake, SQL Server, YugabyteDB |
| **Cloud Platforms** | AWS, Azure, GCP |
| **Container Orchestration** | Kubernetes |
| **Identity Providers** | Okta, OneLogin, JumpCloud, Google |
| **Monitoring** | Datadog, Splunk, Elasticsearch, Coralogix |
| **Message Queues** | RabbitMQ |
| **Network & Security** | SSH, ZeroTier, Cisco NGFW, Fortinet, Palo Alto |

See the [examples directory](https://github.com/adaptive-scale/terraform-provider-adaptive/tree/master/examples) for integration-specific configurations.

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `service_token` (String, Sensitive) Service account token for authenticating with the Adaptive service. If not provided, provider will default to reading token from default adaptive-cli
- `workspace_url` (String) The workspace to use for the provider. If not set, the default workspace will be used app.adaptive.live

## Additional Resources

- [Adaptive Documentation](https://docs.adaptive.live)
- [Provider Examples](https://github.com/adaptive-scale/terraform-provider-adaptive/tree/master/examples)
- [Terraform Registry](https://registry.terraform.io/providers/adaptive-scale/adaptive/latest)
