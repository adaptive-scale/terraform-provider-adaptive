---
page_title: "adaptive_authorization Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
    Provide settings for Adaptive Authorizations
---

# adaptive_authorization (Resource)

Authorizations are granular permissions to adaptive resources that can be granted to adaptive endpoints, that limit the access and reach of the endpoint.
You can create an authorization that allows selective privileges for users when they try to access an endpoint.

```hcl
# postgres_actors is a deployed database which has tables `film` and `actor`
resource "adaptive_authorization" "postgres" {
    name = "select-only-for-actors"
    description = "An example authorization that grants `SELECT` access to film and actor table to a database user named `hg`"
    permission = "GRANT SELECT on film, actor TO hg"
    resource = adaptive_resource.postgres_actors.name
}
```

## Schema

- `name` - (Required) (String) The name of the authorization.
- `resource` - (Required) (String) The name of the resource to associate this authorization with.
- `permission` - (Required) (String) The permission to grant or revoke on the specified resource.
- `description` - (Optional) (String) An optional description of the authorization.

### Read-Only

- `id` (String) The ID of this resource.
