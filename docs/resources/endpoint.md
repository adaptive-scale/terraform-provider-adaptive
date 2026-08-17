---
page_title: "adaptive_endpoint Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
  Manages an Adaptive endpoint, which provides secure access to resources with configurable TTL, JIT access, and user assignments.
---

# adaptive_endpoint (Resource)

The `adaptive_endpoint` resource creates a secure access point to an Adaptive resource. Endpoints are what users connect to when accessing databases, servers, or other services through Adaptive. They provide:

- **Time-limited access** - Configure TTL (time-to-live) for automatic session expiration
- **Just-In-Time (JIT) access** - Require approval before granting access
- **User management** - Assign specific users who can access the endpoint
- **Auto-pause** - Automatically pause inactive endpoints to save resources

## Example Usage

### Basic Endpoint

```terraform
resource "adaptive_endpoint" "basic" {
  name     = "postgres-readonly"
  resource = adaptive_resource.postgres.name
  users    = ["developer@example.com"]
  tags     = ["development"]
}
```

### Endpoint with TTL

```terraform
resource "adaptive_endpoint" "time_limited" {
  name     = "postgres-temp-access"
  resource = adaptive_resource.postgres.name
  ttl      = "4h"  # Access expires after 4 hours
  users    = ["contractor@example.com"]
  tags     = ["temporary"]
}
```

### Endpoint with Just-In-Time Access

```terraform
resource "adaptive_endpoint" "jit_enabled" {
  name           = "production-db-jit"
  resource       = adaptive_resource.production_postgres.name
  is_jit_enabled = true
  jit_approvers  = ["security@example.com", "manager@example.com"]
  users          = ["developer@example.com", "oncall@example.com"]
  tags           = ["production", "jit"]
}
```

### Endpoint with Authorization

```terraform
resource "adaptive_authorization" "readonly" {
  name          = "postgres-readonly"
  resource_type = "postgres"
  permissions   = "SELECT"
}

resource "adaptive_endpoint" "authorized" {
  name          = "postgres-readonly-access"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.readonly.name
  users         = ["analyst@example.com"]
  tags          = ["readonly"]
}
```

### Endpoint with Auto-Pause

```terraform
resource "adaptive_endpoint" "auto_pause" {
  name          = "dev-database"
  resource      = adaptive_resource.dev_postgres.name
  pause_timeout = "30m"  # Pause after 30 minutes of inactivity
  users         = ["developer@example.com"]
  tags          = ["development"]
}
```

### Kubernetes Endpoint with Custom Resources

```terraform
resource "adaptive_endpoint" "k8s_endpoint" {
  name     = "k8s-admin"
  resource = adaptive_resource.k8s_cluster.name
  type     = "kubernetes"
  cpu      = "500m"
  memory   = "512Mi"
  users    = ["platform-team@example.com"]
  tags     = ["kubernetes", "admin"]
}
```

### Multiple Users with Different Access Patterns

```terraform
# Development endpoint - longer TTL, no JIT
resource "adaptive_endpoint" "dev_access" {
  name     = "dev-database"
  resource = adaptive_resource.dev_db.name
  ttl      = "8h"
  users    = ["dev-team@example.com"]
  tags     = ["development"]
}

# Production endpoint - short TTL, JIT required
resource "adaptive_endpoint" "prod_access" {
  name           = "prod-database"
  resource       = adaptive_resource.prod_db.name
  ttl            = "2h"
  is_jit_enabled = true
  jit_approvers  = ["security@example.com"]
  users          = ["oncall@example.com"]
  tags           = ["production"]
}
```

### Complete Example with Resource and Group

```terraform
# Create the resource
resource "adaptive_resource" "postgres" {
  name          = "app-database"
  type          = "postgres"
  host          = "postgres.example.com"
  port          = "5432"
  username      = "admin"
  password      = var.db_password
  database_name = "myapp"
}

# Create authorization for read-only access
resource "adaptive_authorization" "readonly" {
  name          = "readonly-access"
  resource_type = "postgres"
  permissions   = "SELECT"
}

# Create endpoint
resource "adaptive_endpoint" "app_db" {
  name          = "app-database-access"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.readonly.name
  ttl           = "8h"
  users         = ["developer@example.com"]
  tags          = ["app", "database"]
}

# Assign to group
resource "adaptive_group" "developers" {
  name      = "app-developers"
  members   = ["dev1@example.com", "dev2@example.com"]
  endpoints = [adaptive_endpoint.app_db.name]
}
```

### Approval Workflow and Session Hardening

```terraform
resource "adaptive_endpoint" "regulated" {
  name           = "prod-database-regulated"
  resource       = adaptive_resource.prod_db.name
  is_jit_enabled = true
  jit_mode       = "both" # session and script access both need approval

  jit_approvers       = ["security@example.com", "manager@example.com"]
  jit_multi_approver  = true
  jit_total_approvers = 2

  # Hide the in-browser access paths and stop retaining terminal output.
  disable_web_cli        = true
  disable_data_studio    = true
  disable_output_capture = true

  users = ["oncall@example.com"]
  tags  = ["production", "regulated"]
}
```

`disable_output_capture` is OR'd with the workspace-level setting: an endpoint can switch capture off, but it cannot re-enable capture the workspace has disabled. `disable_data_studio` and `disable_web_cli` only affect `cli` endpoints. Set `auto_approval` to approve Just-In-Time requests without a human in the loop.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the session to create.
- `resource` (String) The resource used to create the session.

### Optional

- `authorization` (String) The authorization to use when creating the session.
- `auto_approval` (Boolean) Automatically approve Just-In-Time access requests.
- `cluster` (String) The cluster in which this session should be created. If not provided will be set to default cluster set in workspace settings of the user's workspace
- `cpu` (String) CPU of endpoint pod
- `disable_data_studio` (Boolean) Hide Data Studio for this endpoint. Only meaningful for `cli` endpoints.
- `disable_output_capture` (Boolean) Do not retain terminal output for this endpoint. The effective value is OR'd with the workspace-level setting: an endpoint cannot re-enable capture that the workspace disabled.
- `disable_web_cli` (Boolean) Hide "Connect via Web CLI" for this endpoint. Only meaningful for `cli` endpoints.
- `groups` (List of String) The list of groups associated with the adaptive endpoint
- `idle_timeout` (String) The time after which the session will be automatically terminated if no user is connected. Defaults to never timeout.
- `is_jit_enabled` (Boolean) Whether Just-In-Time access is enabled for the session
- `jit_approvers` (List of String) The list of user emails who can approve Just-In-Time access requests
- `jit_mode` (String) Which access paths require Just-In-Time approval. One of: session, script, both.
- `jit_multi_approver` (Boolean) Require more than one approver for Just-In-Time access requests.
- `jit_total_approvers` (Number) Number of approvals required when `jit_multi_approver` is enabled.
- `memory` (String) Memory of endpoint pod
- `pause_timeout` (String) The time after which the session will be paused if no user has connected to it. Defaults to never pause.
- `script_only_access` (Boolean) Whether the endpoint should only be accessible via script
- `tags` (List of String) Optional tags
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `ttl` (String) The time-to-live (TTL) for the session. The session will be automatically terminated after this time period. If not set, defaults to 90 days.
- `type` (String) The type of session to create. One of: direct (the default), cli, client, services. `direct` and `cli` are the same session type.
- `users` (List of String) The list of users associated with the adaptive endpoint

### Read-Only

- `expose_status` (String) Provisioning status of the service exposure.
- `expose_type` (String) How the endpoint is exposed (e.g. LoadBalancer).
- `exposed` (Boolean) Whether the endpoint is exposed as a service.
- `id` (String) The ID of this resource.
- `status` (String) Current lifecycle status of the endpoint.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String)
- `delete` (String)
- `update` (String)

## TTL Format

The `ttl` field accepts duration strings in the following formats:
- Hours: `1h`, `8h`, `24h`
- Minutes: `30m`, `60m`
- Combined: `1h30m`
- Days: `1d`, `7d` (equivalent to `24h`, `168h`)

## Import

Endpoints can be imported using the endpoint ID:

```shell
terraform import adaptive_endpoint.example endpoint-id
```

Import reads the full endpoint back from the platform, so the arguments above are populated from what the server holds. Refresh is more conservative: optional arguments your configuration never set (`cluster`, `memory`, `cpu`, `ttl`) keep their workspace defaults rather than being written into state, and a toggle that is off locally is never flipped on by a server-side value, since several of them are OR'd with workspace settings.
