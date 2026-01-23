---
page_title: "adaptive_authorization Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
  Manages an Adaptive authorization, which defines permission policies for accessing resources.
---

# adaptive_authorization (Resource)

The `adaptive_authorization` resource defines permission policies that control what actions users can perform on resources. Authorizations are applied to endpoints to enforce least-privilege access.

## Supported Resource Types

| Type | Description | Permission Format |
|------|-------------|-------------------|
| `postgres` | PostgreSQL database | YAML with database, privileges, objects |
| `mysql` | MySQL database | YAML with objects (database.table), privileges |
| `sqlserver` | Microsoft SQL Server | YAML with server_roles, server_permissions, databases |
| `mongodb` | MongoDB database | JSON with role, privileges, actions |
| `kubernetes` | Kubernetes cluster | Kubernetes RBAC Role manifest |
| `ssh` | SSH server | YAML with allow/deny for data_transfer, operations, directories |

## Example Usage

### PostgreSQL Read-Only Access

```terraform
resource "adaptive_authorization" "postgres_readonly" {
  name          = "postgres-readonly"
  resource_type = "postgres"
  permissions   = <<-EOT
    allow:
      - database: production
        privileges:
          - SELECT
        objects:
          - ALL
  EOT
  description   = "Read-only access to PostgreSQL databases"
}
```

### PostgreSQL Read-Write Access

```terraform
resource "adaptive_authorization" "postgres_readwrite" {
  name          = "postgres-readwrite"
  resource_type = "postgres"
  permissions   = <<-EOT
    allow:
      - database: application
        privileges:
          - SELECT
          - INSERT
          - UPDATE
          - DELETE
        objects:
          - ALL
  EOT
  description   = "Read-write access to PostgreSQL databases"
}
```

### PostgreSQL Admin Access

```terraform
resource "adaptive_authorization" "postgres_admin" {
  name          = "postgres-admin"
  resource_type = "postgres"
  permissions   = <<-EOT
    allow:
      - database: sakila
        privileges:
          - ALL
        objects:
          - ALL
  EOT
  description   = "Full administrative access to PostgreSQL"
}
```

### PostgreSQL Multi-Database Access

```terraform
resource "adaptive_authorization" "postgres_multi_db" {
  name          = "postgres-multi-db"
  resource_type = "postgres"
  permissions   = <<-EOT
    allow:
      - database: sakila
        privileges:
          - ALL
        objects:
          - ALL
      - database: employees
        privileges:
          - SELECT
          - INSERT
          - UPDATE
        objects:
          - departments
          - employees
      - database: analytics
        privileges:
          - SELECT
        objects:
          - reports
          - dashboards
  EOT
  description   = "Access to multiple databases with different permissions"
}
```

### MySQL Read-Only Access

```terraform
resource "adaptive_authorization" "mysql_readonly" {
  name          = "mysql-readonly"
  resource_type = "mysql"
  permissions   = <<-EOT
    allow:
      - objects:
          - production.*
        privileges:
          - SELECT
  EOT
  description   = "Read-only access to MySQL databases"
}
```

### MySQL Read-Write Access

```terraform
resource "adaptive_authorization" "mysql_readwrite" {
  name          = "mysql-readwrite"
  resource_type = "mysql"
  permissions   = <<-EOT
    allow:
      - objects:
          - application.*
        privileges:
          - SELECT
          - INSERT
          - UPDATE
          - DELETE
  EOT
  description   = "Read-write access to MySQL databases"
}
```

### MySQL Table-Specific Access

```terraform
resource "adaptive_authorization" "mysql_tables" {
  name          = "mysql-table-access"
  resource_type = "mysql"
  permissions   = <<-EOT
    allow:
      - objects:
          - ecommerce.orders
          - ecommerce.products
        privileges:
          - SELECT
          - INSERT
          - UPDATE
      - objects:
          - ecommerce.users
        privileges:
          - SELECT
  EOT
  description   = "Access to specific MySQL tables with different permissions"
}
```

### SQL Server Read-Only Access

```terraform
resource "adaptive_authorization" "sqlserver_readonly" {
  name          = "sqlserver-readonly"
  resource_type = "sqlserver"
  permissions   = <<-EOT
    name: readonly-sqlserver
    databases:
      include_databases:
        - production
      database_roles:
        - db_datareader
  EOT
  description   = "Read-only access to SQL Server databases"
}
```

### SQL Server Read-Write Access

```terraform
resource "adaptive_authorization" "sqlserver_readwrite" {
  name          = "sqlserver-readwrite"
  resource_type = "sqlserver"
  permissions   = <<-EOT
    name: readwrite-sqlserver
    databases:
      include_databases:
        - application
      database_roles:
        - db_datareader
        - db_datawriter
  EOT
  description   = "Read-write access to SQL Server databases"
}
```

### SQL Server Admin with Server Roles

```terraform
resource "adaptive_authorization" "sqlserver_admin" {
  name          = "sqlserver-admin"
  resource_type = "sqlserver"
  permissions   = <<-EOT
    name: admin-sqlserver
    server_roles:
      - securityadmin
    server_permissions:
      - effect: ALLOW
        action: ALTER ANY LOGIN
    databases:
      include_databases:
        - nwnd
      database_roles:
        - db_datareader
      permissions:
        - effect: ALLOW
          action: INSERT
          object:
            schema: dbo
            name: Order Details
  EOT
  description   = "SQL Server admin with server roles and granular permissions"
}
```

### SQL Server Object-Level Permissions

```terraform
resource "adaptive_authorization" "sqlserver_granular" {
  name          = "sqlserver-granular"
  resource_type = "sqlserver"
  permissions   = <<-EOT
    name: granular-sqlserver
    databases:
      include_databases:
        - ecommerce
      permissions:
        - effect: ALLOW
          action: SELECT
          object:
            schema: dbo
            name: Products
        - effect: ALLOW
          action: SELECT
          object:
            schema: dbo
            name: Categories
        - effect: ALLOW
          action: INSERT
          object:
            schema: dbo
            name: Orders
        - effect: ALLOW
          action: UPDATE
          object:
            schema: dbo
            name: Orders
        - effect: DENY
          action: DELETE
          object:
            schema: dbo
            name: Orders
  EOT
  description   = "SQL Server with granular object-level permissions"
}
```

### MongoDB Read-Only Access

```terraform
resource "adaptive_authorization" "mongodb_readonly" {
  name          = "mongodb-readonly"
  resource_type = "mongodb"
  permissions   = <<-EOT
    {
      "role": "readOnly",
      "privileges": [
        {
          "resource": { "db": "production", "collection": "" },
          "actions": ["find", "listCollections", "listIndexes"]
        }
      ],
      "roles": []
    }
  EOT
  description   = "Read-only access to MongoDB collections"
}
```

### MongoDB Read-Write Access

```terraform
resource "adaptive_authorization" "mongodb_readwrite" {
  name          = "mongodb-readwrite"
  resource_type = "mongodb"
  permissions   = <<-EOT
    {
      "role": "readWrite",
      "privileges": [
        {
          "resource": { "db": "application", "collection": "" },
          "actions": ["find", "insert", "update", "remove"]
        }
      ],
      "roles": []
    }
  EOT
  description   = "Read-write access to MongoDB collections"
}
```

### MongoDB Collection-Specific Access

```terraform
resource "adaptive_authorization" "mongodb_collection" {
  name          = "mongodb-collection-access"
  resource_type = "mongodb"
  permissions   = <<-EOT
    {
      "role": "collectionRole",
      "privileges": [
        {
          "resource": { "db": "test", "collection": "prod" },
          "actions": ["find", "update", "insert"]
        },
        {
          "resource": { "db": "test", "collection": "staging" },
          "actions": ["find"]
        }
      ],
      "roles": []
    }
  EOT
  description   = "Collection-specific access with different permissions per collection"
}
```

### MongoDB with Inherited Roles

```terraform
resource "adaptive_authorization" "mongodb_custom" {
  name          = "mongodb-custom-roles"
  resource_type = "mongodb"
  permissions   = <<-EOT
    {
      "role": "appAdmin",
      "privileges": [
        {
          "resource": { "db": "admin", "collection": "system.users" },
          "actions": ["find", "update"]
        }
      ],
      "roles": [
        { "role": "read", "db": "analytics" },
        { "role": "readWrite", "db": "application" },
        { "role": "dbAdmin", "db": "application" }
      ]
    }
  EOT
  description   = "Custom MongoDB role with inherited roles across databases"
}
```

### Kubernetes Namespace Admin

```terraform
resource "adaptive_authorization" "k8s_namespace_admin" {
  name          = "k8s-namespace-admin"
  resource_type = "kubernetes"
  permissions   = <<-EOT
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: adaptive-admin
    rules:
    - apiGroups: ["*"]
      resources: ["*"]
      verbs: ["*"]
  EOT
  description   = "Full access within a Kubernetes namespace"
}
```

### Kubernetes Read-Only

```terraform
resource "adaptive_authorization" "k8s_readonly" {
  name          = "k8s-readonly"
  resource_type = "kubernetes"
  permissions   = <<-EOT
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: adaptive-readonly
    rules:
    - apiGroups: [""]
      resources: ["pods", "services", "configmaps"]
      verbs: ["get", "list", "watch"]
    - apiGroups: ["apps"]
      resources: ["deployments", "replicasets"]
      verbs: ["get", "list", "watch"]
  EOT
  description   = "Read-only access to core Kubernetes resources"
}
```

### Kubernetes Developer Access

```terraform
resource "adaptive_authorization" "k8s_developer" {
  name          = "k8s-developer"
  resource_type = "kubernetes"
  permissions   = <<-EOT
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: adaptive-developer
    rules:
    - apiGroups: [""]
      resources: ["pods", "services", "configmaps", "secrets"]
      verbs: ["get", "list", "watch", "create", "update", "delete"]
    - apiGroups: ["apps"]
      resources: ["deployments", "replicasets", "statefulsets"]
      verbs: ["get", "list", "watch", "create", "update", "delete"]
    - apiGroups: [""]
      resources: ["pods/log", "pods/exec"]
      verbs: ["get", "create"]
  EOT
  description   = "Developer access to manage workloads in Kubernetes"
}
```

### SSH Restricted Access

```terraform
resource "adaptive_authorization" "ssh_restricted" {
  name          = "ssh-restricted"
  resource_type = "ssh"
  permissions   = <<-EOT
    allow:
      data_transfer:
        - upload
        - download
        - list
    deny:
      operations:
        - cp
        - rm
        - cat
        - env
        - mv
        - vi
      directories:
        - /opt
        - /var
  EOT
  description   = "SSH access with restricted operations and directory access"
}
```

### SSH Read-Only Access

```terraform
resource "adaptive_authorization" "ssh_readonly" {
  name          = "ssh-readonly"
  resource_type = "ssh"
  permissions   = <<-EOT
    allow:
      data_transfer:
        - download
        - list
    deny:
      operations:
        - rm
        - mv
        - cp
        - vi
        - nano
        - dd
        - chmod
        - chown
      directories:
        - /etc
        - /root
        - /var/log
  EOT
  description   = "SSH read-only access for monitoring and log viewing"
}
```

### SSH Developer Access

```terraform
resource "adaptive_authorization" "ssh_developer" {
  name          = "ssh-developer"
  resource_type = "ssh"
  permissions   = <<-EOT
    allow:
      data_transfer:
        - upload
        - download
        - list
      directories:
        - /home
        - /app
        - /var/www
    deny:
      operations:
        - rm -rf
        - dd
        - mkfs
        - fdisk
      directories:
        - /etc
        - /root
        - /boot
  EOT
  description   = "SSH developer access with safe boundaries"
}
```

### Using Authorization with Endpoint

```terraform
# Create authorization
resource "adaptive_authorization" "readonly" {
  name          = "app-readonly"
  resource_type = "postgres"
  permissions   = "SELECT"
  description   = "Read-only access for analytics queries"
}

# Create resource
resource "adaptive_resource" "postgres" {
  name          = "analytics-db"
  type          = "postgres"
  host          = "postgres.example.com"
  port          = "5432"
  username      = "admin"
  password      = var.db_password
  database_name = "analytics"
}

# Create endpoint with authorization
resource "adaptive_endpoint" "analytics_readonly" {
  name          = "analytics-readonly"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.readonly.name
  users         = ["analyst@example.com"]
  tags          = ["analytics", "readonly"]
}
```

### Multiple Authorization Levels

```terraform
# Read-only authorization
resource "adaptive_authorization" "readonly" {
  name          = "db-readonly"
  resource_type = "postgres"
  permissions   = "SELECT"
  description   = "Read-only access"
}

# Read-write authorization
resource "adaptive_authorization" "readwrite" {
  name          = "db-readwrite"
  resource_type = "postgres"
  permissions   = "SELECT, INSERT, UPDATE, DELETE"
  description   = "Read-write access"
}

# Admin authorization
resource "adaptive_authorization" "admin" {
  name          = "db-admin"
  resource_type = "postgres"
  permissions   = "ALL PRIVILEGES"
  description   = "Administrative access"
}

# Endpoints with different authorization levels
resource "adaptive_endpoint" "readonly_endpoint" {
  name          = "db-readonly-access"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.readonly.name
  users         = ["analyst@example.com"]
}

resource "adaptive_endpoint" "readwrite_endpoint" {
  name          = "db-readwrite-access"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.readwrite.name
  users         = ["developer@example.com"]
}

resource "adaptive_endpoint" "admin_endpoint" {
  name          = "db-admin-access"
  resource      = adaptive_resource.postgres.name
  authorization = adaptive_authorization.admin.name
  users         = ["dba@example.com"]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the authorization object.
- `permissions` (String) The permission to grant or revoke on the specified resource.
- `resource_type` (String) Resource type to grant permission on. Eg. kubernetes, postgres, mysql, mongodb

### Optional

- `description` (String) An optional description of the authorization object.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Authorizations can be imported using the authorization ID:

```shell
terraform import adaptive_authorization.example authorization-id
```
