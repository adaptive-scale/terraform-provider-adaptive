---
page_title: "adaptive_endpoint Endpoint - terraform-provider-adaptive"
subcategory: ""
description: |-
  Settings for Adaptive Endpoints
---

# adaptive_endpoint (Resource)

An endpoint is a endpoint through which resource can be accessed via adaptive.

## Example Usage

```hcl
resource "adaptive_resource" "postgres_test" {
    type          = "postgres"
    name          = "postgres-test"
    host          = "example.com"
    port          = "5432"
    username      = "postgresas"
    password      = "postgrespassword"
    database_name = "postgresadmin"
}

resource "adaptive_endpoint" "session" {
  name         = "session"
  type = "direct"
  ttl          = "3h"
  resource = adaptive_resource.postgres_test.name
  users = ["qa@adaptive.live", "sales@adaptive.live"]
}
```

## Schema

The following arguments are supported by each individual adaptive-endpoints:

- `name` - (Required) The name of this endpoint.
- `resource` - (Required) The adaptive resource used to create the endpoint.
- `type` - (Optional) The type of endpoint to create. Defaults to "direct".
- `ttl` - (Required) Duration for which endpoint will remain active.
- `authorization` - (Optional) Name of authorization to assign to the created endpoint. Currently authorizations are only supported by Postgres and Kubernetes resources.
- `cluster` - (Optional) Name of adaptive-resource cluster in which this endpoint should be created. If not provided, default cluster set in workspace settings of the user's workspace would be used.
- `users` - (Optional) List of emails, to make members of this adaptive session. Emails must exist as users in your workspace. By default, creators would be member of the endpoint.

### Read-Only

- `id` (String) The ID of this resource.
