---
page_title: "adaptive_script Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
  Manages an Adaptive script, which executes commands on endpoints for automation.
---

# adaptive_script (Resource)

The `adaptive_script` resource stores reusable command definitions for later execution on Adaptive endpoints. Scripts are useful for:

- **Automation** - Run setup scripts, migrations, or maintenance tasks
- **Provisioning** - Initialize databases, create users, or set up schemas
- **Operations** - Prepare operational commands for later invocation

## Lifecycle and secret handling

Terraform stores the script definition; creating or updating this resource does not execute the command. Invocation, endpoint readiness and the command's runtime result are separate operations. The endpoint binding cannot be changed in place and is rejected during planning and apply rather than replaced automatically.

The server may withhold the command body. Import cannot recover it; configure the intended command before an update. Sensitive command values can still persist in Terraform state and saved plans. The last acknowledged command fingerprint is retained across refreshes and missing metadata, so unresolved drift remains observable. A separate post-write fingerprint read is best-effort, not an atomic acknowledgment.

An explicitly empty parameter-description map is sent as `{}` so existing map entries can be removed. The current backend preserves an empty description rather than clearing it; Terraform rejects description removal until backend support exists. Provider sensitivity and sanitized diagnostics cannot prevent logging performed by the backend itself.

## Example Usage

### Basic Script Definition

```terraform
resource "adaptive_script" "init_db" {
  name     = "initialize-database"
  endpoint = adaptive_endpoint.postgres.name
  command  = "CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name VARCHAR(100));"
}
```

### Database Migration Script

```terraform
resource "adaptive_script" "migration" {
  name     = "run-migrations"
  endpoint = adaptive_endpoint.postgres.name
  command  = <<-EOT
    CREATE TABLE IF NOT EXISTS schema_migrations (
      version VARCHAR(255) PRIMARY KEY,
      applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    INSERT INTO schema_migrations (version) VALUES ('001_initial') ON CONFLICT DO NOTHING;
  EOT
}
```

### Create Database User

```terraform
resource "adaptive_script" "create_user" {
  name     = "create-app-user"
  endpoint = adaptive_endpoint.postgres.name
  command  = <<-EOT
    DO $$
    BEGIN
      IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_user') THEN
        CREATE USER app_user WITH PASSWORD 'secure_password';
        GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO app_user;
      END IF;
    END
    $$;
  EOT
}
```

### Kubernetes Namespace Setup

```terraform
resource "adaptive_script" "k8s_setup" {
  name     = "setup-namespace"
  endpoint = adaptive_endpoint.k8s_cluster.name
  command  = "kubectl create namespace app-production --dry-run=client -o yaml | kubectl apply -f -"
}
```

### SSH Server Configuration

```terraform
resource "adaptive_script" "server_setup" {
  name     = "install-packages"
  endpoint = adaptive_endpoint.bastion.name
  command  = "sudo apt-get update && sudo apt-get install -y htop vim curl"
}
```

### MongoDB Index Creation

```terraform
resource "adaptive_script" "create_indexes" {
  name     = "create-mongodb-indexes"
  endpoint = adaptive_endpoint.mongodb.name
  command  = <<-EOT
    db.users.createIndex({ email: 1 }, { unique: true });
    db.orders.createIndex({ userId: 1, createdAt: -1 });
    db.products.createIndex({ category: 1, price: 1 });
  EOT
}
```

### Multiple Scripts with Dependencies

```terraform
# First: Create the schema
resource "adaptive_script" "create_schema" {
  name     = "create-schema"
  endpoint = adaptive_endpoint.postgres.name
  command  = "CREATE SCHEMA IF NOT EXISTS app;"
}

# Second: Create tables (depends on schema)
resource "adaptive_script" "create_tables" {
  name     = "create-tables"
  endpoint = adaptive_endpoint.postgres.name
  command  = <<-EOT
    CREATE TABLE IF NOT EXISTS app.users (
      id SERIAL PRIMARY KEY,
      email VARCHAR(255) UNIQUE NOT NULL,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS app.orders (
      id SERIAL PRIMARY KEY,
      user_id INTEGER REFERENCES app.users(id),
      total DECIMAL(10,2),
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
  EOT

  depends_on = [adaptive_script.create_schema]
}

# Third: Create indexes (depends on tables)
resource "adaptive_script" "create_indexes" {
  name     = "create-indexes"
  endpoint = adaptive_endpoint.postgres.name
  command  = <<-EOT
    CREATE INDEX IF NOT EXISTS idx_users_email ON app.users(email);
    CREATE INDEX IF NOT EXISTS idx_orders_user ON app.orders(user_id);
    CREATE INDEX IF NOT EXISTS idx_orders_created ON app.orders(created_at DESC);
  EOT

  depends_on = [adaptive_script.create_tables]
}
```

### Complete Example

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

# Create the endpoint
resource "adaptive_endpoint" "postgres" {
  name     = "app-database-admin"
  resource = adaptive_resource.postgres.name
  users    = ["admin@example.com"]
}

# Run initialization scripts
resource "adaptive_script" "init" {
  name     = "initialize-app-database"
  endpoint = adaptive_endpoint.postgres.name
  command  = <<-EOT
    -- Create application schema
    CREATE SCHEMA IF NOT EXISTS app;

    -- Create users table
    CREATE TABLE IF NOT EXISTS app.users (
      id SERIAL PRIMARY KEY,
      email VARCHAR(255) UNIQUE NOT NULL,
      name VARCHAR(100),
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    -- Create audit log table
    CREATE TABLE IF NOT EXISTS app.audit_log (
      id SERIAL PRIMARY KEY,
      user_id INTEGER,
      action VARCHAR(50),
      details JSONB,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
  EOT
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `command` (String, Sensitive) The command the script runs. Write-only: the Adaptive API never returns script bodies, so this value cannot be read back and an import leaves it empty.
- `endpoint` (String) The endpoint the script is attached to. Cannot be changed after creation.
- `name` (String) The name of the script.

### Optional

- `description` (String) An optional description of the script. The current backend preserves empty descriptions on update, so clearing an existing description is rejected at plan until backend support is available.
- `parameter_descriptions` (Map of String) Descriptions for the script's parameters, keyed by parameter name.

### Read-Only

- `command_digest` (String) Opaque server fingerprint of the script body, used to detect out-of-band changes to the command on refresh.
- `id` (String) The ID of this resource.

## Import

Scripts can be imported using the script ID:

```shell
terraform import adaptive_script.example script-id
```

The Adaptive API does not return script bodies, so `command` comes back empty on import and has to be written into your configuration before the first apply. For the same reason a command edited outside Terraform cannot be read back; the provider instead records the server's fingerprint of the body and plans a rewrite once it stops matching.
