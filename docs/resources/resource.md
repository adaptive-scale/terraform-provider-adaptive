---
page_title: "adaptive_resource Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
  Manages an Adaptive resource, which represents a connection to external services like databases, cloud platforms, and APIs.
---

# adaptive_resource (Resource)

The `adaptive_resource` resource allows you to create and manage connections to external services in Adaptive. Resources are the foundation of Adaptive's access management - they define how to connect to databases, cloud platforms, Kubernetes clusters, and other services.

## Supported Resource Types

| Type | Description |
|------|-------------|
| `postgres` | PostgreSQL database |
| `mysql` | MySQL database |
| `mongodb` | MongoDB database |
| `cockroachdb` | CockroachDB database |
| `clickhouse` | ClickHouse analytics database |
| `snowflake` | Snowflake data warehouse |
| `sqlserver` | Microsoft SQL Server |
| `yugabytedb` | YugabyteDB distributed SQL |
| `aws` | Amazon Web Services |
| `azure` | Microsoft Azure |
| `gcp` | Google Cloud Platform |
| `kubernetes` | Kubernetes cluster |
| `ssh` | SSH server |
| `rdp_windows` | Windows server (single-target RDP) |
| `adaptive_rdp` | Windows RDP fleet — multiple targets, per-target credentials, in-browser access |
| `okta` | Okta identity provider |
| `azureactivedirectory` | Azure Active Directory |
| `onelogin` | OneLogin identity provider |
| `jumpcloud` | JumpCloud directory |
| `google` | Google Workspace |
| `msteams` | Microsoft Teams |
| `datadog` | Datadog monitoring |
| `splunk` | Splunk logging |
| `elasticsearch` | Elasticsearch |
| `coralogix` | Coralogix logging |
| `syslog` | Syslog server |
| `rabbitmq` | RabbitMQ message broker |
| `zerotier` | ZeroTier network |
| `services` | Generic services (URL list) |
| `customintegration` | Custom integration |
| `redis` | Redis / Redis Enterprise Cloud |
| `awselasticcache` | AWS ElastiCache |
| `oracle` | Oracle Database |
| `proxysql` | ProxySQL (with admin interface) |
| `mongodb36` | MongoDB 3.6 |
| `mongodb-do` | MongoDB on DigitalOcean |
| `azure_documentdb` | Azure Cosmos DB for MongoDB |
| `big_query` | Google BigQuery |
| `kafka` | Apache Kafka |
| `confluent` | Confluent Cloud |
| `ldap` | LDAP directory |
| `rdpldap` | Windows RDP with LDAP authentication |
| `vnc` | VNC hosts |
| `windows-server-groups` | Windows server group |
| `chrome` | Automated browser session |
| `1password` | 1Password secrets |
| `heroku` | Heroku |
| `digitalocean` | DigitalOcean |
| `slack_webhook` | Slack incoming webhook |
| `msteams_workflow` | Microsoft Teams workflow webhook |
| `awscloudwatchlogs` | AWS CloudWatch Logs |
| `securetunnels` | Adaptive secure tunnels |

The full list of accepted values is validated by the provider; `terraform plan` names them all if you get one wrong.

## Integration-specific configuration

### MySQL SSL modes

For `type = "mysql"`, explicit `ssl_mode` values are `verify_identity`, `verify_ca`, `required`, `preferred`, or `disabled`. The backend forwards the token unchanged to `mysql --ssl-mode`; PostgreSQL spellings (`verify-full`, `verify-ca`, `allow`, `prefer`, and `disable`) are rejected during planning and before mutation. Use the canonical MySQL spellings instead. The sole legacy prefix `require` remains accepted because the MySQL CLI recognizes it as `REQUIRED`; it is preserved literally on write and refresh, not normalized to `required`. This compatibility exception does not establish equivalence across all backend TLS/certificate paths. A genuinely absent mode retains the historical `require` default; explicit `ssl_mode = ""` clears the setting so the backend omits the CLI SSL-mode flag. Omission on an existing resource preserves an observed mode, and `ignore_changes` continues to honor the planned value. These mappings do not guarantee live TLS negotiation or certificate validation.

### ClickHouse

Use `database_name` for the database and `use_tls` for transport security. These map to the backend's `database` and `use-tls` keys. The old `ssl_mode` argument was not consumed by this integration; nonempty values are now rejected during planning and before a write rather than silently implying a PostgreSQL-style TLS policy. Remove it and set `use_tls` explicitly. A boolean TLS switch does not expose separate `require`, `verify-ca`, or `verify-full` modes.

For both ClickHouse and MongoDB 3.6, `use_tls` is optional and computed: omission retains the observed setting on refresh/import and unrelated updates. Set `use_tls = false` explicitly to disable it; removing the argument no longer requests a reset to false. Existing resources with an explicit boolean retain that desired-state ownership.

### Aruba Instant On

Use `host` (the existing Terraform argument) or `hostname` for the device address. Both map to the backend's `hostname`; differing nonempty aliases are rejected. Imports default to `host`, while established managed alias ownership is retained on refresh. Use `username` with `password` or `key`; `key` selects SSH-key mode. The provider also maps `port`, `webui_port`, and `login_url` to their canonical backend fields.

The old `api_token`/`apiToken` path has no consumed backend field and is now rejected rather than silently discarded. The backend form's `cloud_token` label does not establish runtime API-token support. Managed-browser automation fields are a separate planned expansion, not included here.

### Redshift

`ssl_mode` is forwarded as `sslMode` and refreshed from the same backend key. Runtime enforcement is backend-path dependent: the inspected PostgreSQL proxy path honors this value, while other Redshift execution paths still hardcode `require`. Provider mapping coverage is not a guarantee that every backend connection path implements every TLS mode.

### AWS CLI and Keyspaces

Both integrations forward role and service-account authentication controls as well as static credentials. Public input names remain integration-specific:

| Purpose | AWS CLI (`type = "aws"`) | Keyspaces (`type = "keyspaces"`) |
| --- | --- | --- |
| Region | `region_name` | `aws_region_name` |
| Static access key ID | `access_key_id` | `aws_access_key_id` |
| Static secret access key | `secret_access_key` | `aws_secret_access_key` |
| Role authentication | `use_role_arn`, `aws_role_arn` | `use_role_arn`, `aws_role_arn` |
| Service-account authentication | `use_service_account`, `service_account` | `use_service_account`, `service_account` |

The backend chooses role authentication before service-account authentication when both flags are true. `create_if_not_exists` controls Kubernetes service-account creation on that path, not Keyspaces keyspace creation. These inputs do not make Terraform provision an IAM role or prove that runtime identities have the needed permissions.

Keyspaces also forwards `keyspace`, `keyspaces_endpoint`, and `version`. The shared `service_account` argument remains sensitive for 1Password compatibility; cloud usage is a Kubernetes service-account name. Its global schema and Postgres's `aws_role_arn` ownership were not converted to computed secret inputs.

For these two types only, omitted nonsecret cloud settings retain observed values through bounded SDK diff suppression. Explicit false/empty values request resets, and refresh is not suppressed. This prevents unrelated imported-resource updates from silently changing auth modes or dropping command restrictions. Missing/null read fields preserve observations rather than proving a clear; legacy/incomplete response coverage still limits omission-as-removal detection. Held static credentials retain the separate resupply/explicit-clear guard.

AWS CLI also supports `enable_allowed_commands` and comma-separated `allowed_commands`, for example `ec2:describe-instances,s3:ls`. When enabled and both CLI `command` and `parameter` are nonempty strings, the backend parses the request and checks its exact service/subcommand pair against the allowlist. If CLI parameters are absent or either value is missing, empty, or not a string, the backend falls back to an interactive `/bin/bash` shell before allowlist validation, even when enabled. This is not a command-only security boundary. Empty or whitespace-only `allowed_commands` means allow all, not deny all; explicitly clearing the value relaxes the command-request restriction. This is separate from authorization resource scoping and does not itself create a least-privilege IAM policy. Configuration API tests do not verify live enforcement, IAM permissions, or cloud connectivity.

### Expanded IAM, TLS, identity, and device fields

The following integration-specific inputs are supported in both write and refresh mappings. Shared arguments are not automatically meaningful for every integration; unsupported IAM controls and unused legacy aliases are rejected rather than silently ignored.

| Integration | Additional inputs |
| --- | --- |
| MySQL | `auth_mode`, `use_rds_iam`, `use_irsa`, `aws_region`, `aws_role_arn`, `aws_access_key_id`, `aws_secret_access_key`, `aws_service_account`, `root_cert`, `client_cert`, `client_key`, `old_version`; configured `ssl_mode` is now forwarded instead of ignored |
| Postgres | `auth_mode` alongside the existing RDS IAM inputs |
| Kafka | `use_msk_iam`, `use_irsa`, `aws_region`, `aws_role_arn`, `aws_access_key_id`, `aws_secret_access_key`, `aws_service_account`, `bootstrap_servers` (a comma-separated string, not a list) |
| ElastiCache | `use_iam_auth`, `use_irsa`, `aws_role_arn`, `aws_service_account`, `cache_name`, `cache_type`, plus existing region/static credentials and TLS controls |
| MongoDB | `use_tls`, `client_certificate`, `version` |
| DocumentDB | `tls_enabled` |
| Syslog | `tls_enabled`, `tls_ca_cert`, `client_certificate`, `client_key`, `tls_skip_verify` |
| Services | `tls_enabled` |
| SQL Server / Azure SQL Server | `ssl_mode` |
| Google directory | `service_account` (the JSON credentials document, not a file path), `admin_email` |
| Windows RDP | `allow_file_transfer` |

For these integration-specific controls, omitted values retain observations where supported; explicit false/empty values request the corresponding backend reset. MySQL/Postgres `auth_mode` supports `password` or `aws_rds_iam`; an explicit `use_rds_iam` value takes precedence. Omitted IAM pointer switches retain backend inference instead of becoming false. Absent MySQL SSL mode retains the historical `require` default. None of these settings establishes that IAM, TLS validation, or a live connection succeeded.

`aruba_instant_on`, `aruba_sw`, `cisco_ngfw`, `fortinet_ngfw`, `fortinet_analyzer`, `fortinet_manager`, `hpe_switch`, `ivanti`, `juniper_sw`, `paloalto_ngfw`, and `sophos_fw` support their backend SSH/browser fields and shared `automation_mode`, `fields`, and `script`. Browser fields/scripts are sensitive. Explicit empty browser credential strings are serialized so successful readback can acknowledge the clear; subsequent omission does not require resupplying an already-cleared credential. These devices do not support `prestart`. Cisco NGFW, Fortinet NGFW, and HPE retain the existing public `uri` input for backend `login_url`; use that input rather than public `login_url` for those types.

Google service-account inputs are sensitive, but this provider cannot repair the backend exporter's contextual redaction or missing fingerprint coverage. A conservative update guard treats an observed account document as held even without fingerprint metadata. Do not interpret the configuration tests as proof that backend responses never expose credentials.

The unused `app_id`, `database_account`, `database_username`, and `database_password` arguments are deprecated and rejected with canonical-input guidance. `app_key` is supported for Datadog; configurable `version` is supported for Keyspaces and MongoDB rather than being a generic integration-version override. Resource names remain mutable. Integration-type changes are rejected during planning and apply; migrate attached endpoints explicitly instead of relying on destructive replacement.

### Datadog

Use `dd_api_key` for the API key and `app_key` for the separate application key. They are sent as the independent backend fields `dd_api_key` and `dd_app_key`; supplying one never substitutes it for the other. Both public arguments are sensitive. Missing or null credential fields on read preserve prior values; an explicit empty string represents a clear.

Supported backend fingerprint metadata maps an application-key rotation to `app_key`. As with other held credentials, import cannot recover the plaintext key: supply the required matching credentials before an update, or explicitly clear a supported optional credential. Sensitive arguments still persist in Terraform state; this is not a native write-only input.

## Secret Arguments

Credential arguments (`password`, `key`, `api_token`, `client_secret`, `secret_id`, the LDAP and ProxySQL passwords, and the rest) are marked sensitive, so Terraform hides their values in ordinary plan output. For fields correctly classified as secret by the backend, reads withhold the value and report an opaque fingerprint when supported. The provider records those fingerprints as of its last write in the computed `applied_digests` map and compares them on refresh, so supported scalar credential drift can appear on the argument it belongs to without displaying its value. Provider sensitivity does not repair backend redaction gaps or supply missing fingerprints.

The platform stores configuration as a single blob. Before any update the provider re-reads the resource and rejects an update that would omit a credential known to be held by the server. This prevents an unrelated tags-only change after import from silently clearing known credentials:

- An argument supplied in configuration may be reapplied only when the actual planned credential payload agrees with that configuration. If `lifecycle.ignore_changes` retains a drift fingerprint instead, the update is rejected rather than sending the fingerprint or overriding the lifecycle policy. Secret-preserving metadata-only updates require the separately planned backend patch contract.
- A credential supplied for an inactive authentication mode must not silently disappear from the replacement payload; the provider rejects that update unless a supported clear is explicitly authorized.
- Omitting a known held argument is rejected before the mutation request. To intentionally clear a supported optional string credential, configure it explicitly as `""`; required RDP credentials cannot be cleared this way.
- Once the server explicitly reports a credential as an empty string, its old fingerprint is pruned and a later update may omit it. Missing fields, JSON nulls, missing digests and failed reads do not prove a clear.
- Unknown credential mappings are rejected conservatively. The opaque `adaptiveencrypt` resource requires an explicitly configured `value` on update because its read response cannot recover that payload.

Refresh preserves the fingerprint baseline until a successful write, so unresolved changes and warning-only drift remain observable across repeated refreshes. An initial import or older state without a baseline seeds the currently reported fingerprints; that is an observation, not proof Terraform wrote those values. An entirely empty baseline cannot distinguish first initialization from a first externally added secret. Digest reporting still depends on backend redaction coverage and a stable fingerprint key; older/incomplete responses cannot establish that no secrets exist. A separate post-write read is best-effort and is not atomic with the write.

`Sensitive` hides ordinary CLI presentation but does **not** remove these legacy credential values from Terraform state or saved plan artifacts. Protect state, plans, logs, and historical snapshots. Native write-only inputs and secret-preserving backend patch updates are planned separately; they are not provided by this change.

## Example Usage

### PostgreSQL Database

```terraform
resource "adaptive_resource" "postgres" {
  name          = "production-postgres"
  type          = "postgres"
  host          = "postgres.example.com"
  port          = "5432"
  username      = "admin"
  password      = var.db_password
  database_name = "myapp"
  ssl_mode      = "require"
  tags          = ["production", "database"]
}
```

### PostgreSQL on RDS with IAM authentication

Set `use_rds_iam` to authenticate with short-lived AWS RDS IAM auth tokens
instead of a stored password, so no `password` is needed. The database user must
have been granted `rds_iam`, the RDS instance must have IAM authentication
enabled, and `ssl_mode` cannot be `disable`.

`use_irsa` selects the identity that mints the tokens. With it on, tokens are
minted through `aws_service_account` — a Kubernetes ServiceAccount in the
session cluster annotated with `eks.amazonaws.com/role-arn` — or, when that is
empty, through the platform's own IRSA role or instance profile. With it off,
`aws_access_key_id` / `aws_secret_access_key` are used instead. Leaving
`use_irsa` unset infers it from whether static keys were given.

`aws_region` is only needed when the region cannot be derived from an
`*.rds.amazonaws.com` hostname, and `aws_role_arn` is an optional role to assume.

```terraform
resource "adaptive_resource" "rds_postgres" {
  name     = "production-rds"
  type     = "postgres"
  host     = "mydb.abc123.us-east-1.rds.amazonaws.com"
  port     = "5432"
  username = "iam_user"
  ssl_mode = "require"

  use_rds_iam         = true
  use_irsa            = true
  aws_service_account = "adaptive-rds-access"
}

resource "adaptive_resource" "rds_postgres_static_keys" {
  name     = "production-rds-keys"
  type     = "postgres"
  host     = "mydb.abc123.eu-west-1.rds.amazonaws.com"
  port     = "5432"
  username = "iam_user"
  ssl_mode = "require"

  use_rds_iam           = true
  use_irsa              = false
  aws_access_key_id     = var.aws_access_key_id
  aws_secret_access_key = var.aws_secret_access_key
}
```

### MySQL Database

```terraform
resource "adaptive_resource" "mysql" {
  name          = "app-mysql"
  type          = "mysql"
  host          = "mysql.example.com"
  port          = "3306"
  username      = "root"
  password      = var.mysql_password
  database_name = "application"
  ssl_mode      = "REQUIRED"
  tags          = ["staging"]
}
```

### MongoDB

```terraform
resource "adaptive_resource" "mongodb" {
  name = "analytics-mongodb"
  type = "mongodb"
  uri  = "mongodb+srv://user:password@cluster.mongodb.net/analytics"
  tags = ["analytics"]
}
```

### Kubernetes Cluster

```terraform
resource "adaptive_resource" "k8s" {
  name          = "production-cluster"
  type          = "kubernetes"
  api_server    = "https://kubernetes.example.com:6443"
  cluster_cert  = file("ca.crt")
  cluster_token = var.k8s_token
  namespace     = "default"
  tags          = ["production", "k8s"]
}
```

### AWS

```terraform
resource "adaptive_resource" "aws" {
  name              = "aws-production"
  type              = "aws"
  access_key_id     = var.aws_access_key
  secret_access_key = var.aws_secret_key
  region_name       = "us-west-2"
  tags              = ["production", "cloud"]
}
```

### Azure

```terraform
resource "adaptive_resource" "azure" {
  name              = "azure-production"
  type              = "azure"
  tenant_id         = var.azure_tenant_id
  application_id    = var.azure_app_id
  api_client_id     = var.azure_client_id
  api_client_secret = var.azure_client_secret
  tags              = ["production", "cloud"]
}
```

### GCP

```terraform
resource "adaptive_resource" "gcp" {
  name       = "gcp-production"
  type       = "gcp"
  project_id = "my-project-123"
  key_file   = file("service-account.json")
  tags       = ["production", "cloud"]
}
```

### SSH Server

```terraform
resource "adaptive_resource" "ssh" {
  name     = "bastion-host"
  type     = "ssh"
  host     = "bastion.example.com"
  port     = "22"
  username = "admin"
  key      = file("~/.ssh/id_rsa")
  tags     = ["infrastructure"]
}
```

### Windows Server (Single-Target RDP)

```terraform
resource "adaptive_resource" "windows" {
  name     = "windows-server"
  type     = "rdp_windows"
  hostname = "windows.example.com"
  port     = "3389"
  username = "Administrator"
  password = var.windows_password
  tags     = ["infrastructure", "windows"]
}
```

### Adaptive RDP (Multi-Target Fleet)

The `adaptive_rdp` type manages a fleet of Windows RDP hosts in a single resource. Each `targets` block is one host with its own credentials; users pick a target from the in-browser desktop picker. `password` is required (RDP logins with blank passwords fail to authenticate) and marked sensitive, `port` defaults to `3389`, and the optional `record` overrides the global session-recording setting per target (omit it to inherit the global setting). The `targets` block is only accepted on `adaptive_rdp` resources; any other type rejects it.

```terraform
resource "adaptive_resource" "rdp_fleet" {
  name = "rdp-fleet-01"
  type = "adaptive_rdp"

  targets {
    id       = "server-1"
    name     = "Windows Server 1"
    host     = "10.0.0.5"
    port     = 3389
    username = "administrator"
    password = var.win1_password
  }

  targets {
    id       = "server-2"
    name     = "Windows Server 2"
    host     = "10.0.0.6"
    username = "administrator"
    password = var.win2_password
    domain   = "CORP"
    record   = true
  }

  tags = ["windows", "production"]
}
```

### Snowflake

```terraform
resource "adaptive_resource" "snowflake" {
  name      = "analytics-snowflake"
  type      = "snowflake"
  host      = "account.snowflakecomputing.com"
  username  = "TERRAFORM_USER"
  password  = var.snowflake_password
  warehouse = "COMPUTE_WH"
  database_name = "ANALYTICS"
  schema    = "PUBLIC"
  role      = "ACCOUNTADMIN"
  tags      = ["analytics"]
}
```

### CockroachDB

```terraform
resource "adaptive_resource" "cockroachdb" {
  name          = "production-cockroachdb"
  type          = "cockroachdb"
  host          = "cockroach.example.com"
  port          = "26257"
  username      = "admin"
  password      = var.cockroach_password
  database_name = "defaultdb"
  ssl_mode      = "verify-full"
  root_cert     = file("ca.crt")
  tags          = ["production", "database"]
}
```

### ClickHouse

```terraform
resource "adaptive_resource" "clickhouse" {
  name          = "analytics-clickhouse"
  type          = "clickhouse"
  host          = "clickhouse.example.com"
  port          = "9440"
  username      = "default"
  password      = var.clickhouse_password
  database_name = "analytics"
  ssl_mode      = "require"
  tags          = ["analytics"]
}
```

### Services

```terraform
resource "adaptive_resource" "services" {
  name = "internal-services"
  type = "services"
  urls = "https://api.internal.com,https://admin.internal.com,https://dashboard.internal.com"
  tags = ["internal", "web"]
}
```

### Okta

```terraform
resource "adaptive_resource" "okta" {
  name          = "okta-sso"
  type          = "okta"
  domain        = "mycompany.okta.com"
  client_id     = var.okta_client_id
  client_secret = var.okta_client_secret
  tags          = ["identity", "sso"]
}
```

### Azure Active Directory

```terraform
resource "adaptive_resource" "azure_ad" {
  name          = "azure-ad-integration"
  type          = "azureactivedirectory"
  domain        = "mycompany.onmicrosoft.com"
  tenant_id     = var.azure_tenant_id
  client_id     = var.azure_client_id
  client_secret = var.azure_client_secret
  tags          = ["identity", "azure"]
}
```

### OneLogin

```terraform
resource "adaptive_resource" "onelogin" {
  name              = "onelogin-sso"
  type              = "onelogin"
  domain            = "mycompany.onelogin.com"
  client_id         = var.onelogin_client_id
  client_secret     = var.onelogin_client_secret
  api_client_id     = var.onelogin_api_client_id
  api_client_secret = var.onelogin_api_client_secret
  tags              = ["identity", "sso"]
}
```

### JumpCloud

```terraform
resource "adaptive_resource" "jumpcloud" {
  name          = "jumpcloud-directory"
  type          = "jumpcloud"
  domain        = "console.jumpcloud.com"
  client_id     = var.jumpcloud_client_id
  client_secret = var.jumpcloud_client_secret
  api_token     = var.jumpcloud_api_key
  tags          = ["identity", "directory"]
}
```

### Google Workspace

```terraform
resource "adaptive_resource" "google_workspace" {
  name          = "google-workspace"
  type          = "google"
  domain        = "mycompany.com"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  tags          = ["identity", "google"]
}
```

### Datadog

```terraform
resource "adaptive_resource" "datadog" {
  name       = "datadog-monitoring"
  type       = "datadog"
  dd_site    = "datadoghq.com"
  dd_api_key = var.datadog_api_key
  tags       = ["monitoring"]
}
```

### Splunk

```terraform
resource "adaptive_resource" "splunk" {
  name     = "splunk-logging"
  type     = "splunk"
  url      = "https://splunk.example.com:8088"
  token_id = var.splunk_hec_token
  tags     = ["logging"]
}
```

### Elasticsearch

```terraform
resource "adaptive_resource" "elasticsearch" {
  name     = "elasticsearch-cluster"
  type     = "elasticsearch"
  uri      = "https://elasticsearch.example.com:9200"
  username = "elastic"
  password = var.elasticsearch_password
  index    = "logs"
  tags     = ["logging", "search"]
}
```

### Coralogix

```terraform
resource "adaptive_resource" "coralogix" {
  name             = "coralogix-logging"
  type             = "coralogix"
  uri              = "https://api.coralogix.com"
  private_key      = var.coralogix_private_key
  application_name = "my-application"
  sub_system_name  = "production"
  tags             = ["logging"]
}
```

### Syslog

```terraform
resource "adaptive_resource" "syslog" {
  name     = "syslog-server"
  type     = "syslog"
  hostname = "syslog.example.com"
  port     = "514"
  protocol = "tcp"
  tags     = ["logging"]
}
```

### RabbitMQ

```terraform
resource "adaptive_resource" "rabbitmq" {
  name     = "rabbitmq-broker"
  type     = "rabbitmq"
  uri      = "amqp://rabbitmq.example.com:5672"
  username = "admin"
  password = var.rabbitmq_password
  tags     = ["messaging"]
}
```

### Microsoft Teams

```terraform
resource "adaptive_resource" "msteams" {
  name          = "teams-integration"
  type          = "msteams"
  tenant_id     = var.teams_tenant_id
  client_id     = var.teams_app_id
  client_secret = var.teams_app_key
  tags          = ["collaboration"]
}
```

### ZeroTier

```terraform
resource "adaptive_resource" "zerotier" {
  name       = "zerotier-network"
  type       = "zerotier"
  network_id = "1234567890abcdef"
  api_token  = var.zerotier_api_token
  tags       = ["networking"]
}
```

### Custom Integration

```terraform
resource "adaptive_resource" "custom" {
  name                 = "custom-app"
  type                 = "customintegration"
  image                = "myregistry/myapp:latest"
  service_account_name = "custom-sa"
  tags                 = ["custom"]
}
```

### Reading Secrets from Files

```terraform
locals {
  secrets = yamldecode(file("secrets.yaml"))
}

resource "adaptive_resource" "postgres_from_file" {
  name          = "postgres-secure"
  type          = "postgres"
  host          = local.secrets.postgres.host
  port          = local.secrets.postgres.port
  username      = local.secrets.postgres.username
  password      = local.secrets.postgres.password
  database_name = local.secrets.postgres.database
  ssl_mode      = "require"
}
```

### Using AWS Secrets Manager

```terraform
resource "adaptive_resource" "postgres_with_secrets" {
  name       = "postgres-secrets-manager"
  type       = "postgres"
  host       = "postgres.example.com"
  port       = "5432"
  secret_id  = "arn:aws:secretsmanager:us-west-2:123456789:secret:db-creds"
  aws_arn    = "arn:aws:iam::123456789:role/secrets-access"
  aws_region_name = "us-west-2"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of the Adaptive resource
- `type` (String) Type of the Adaptive resource

### Optional

- `access_control_group` (String) ElastiCache user-group id. Used by the awselasticcache resource.
- `access_control_method` (String) ElastiCache access-control method. Used by the awselasticcache resource.
- `access_key_id` (String, Sensitive) The AWS access key id. Used by AWS resource.
- `admin_email` (String) Google directory delegated administrator email.
- `allow_file_transfer` (Boolean) Allow Windows RDP file transfer; explicit false disables it.
- `allowed_commands` (String) AWS CLI only: comma-separated service:subcommand entries, used when enable_allowed_commands is true. Empty means allow all, NOT deny all.
- `annotations` (String) The annotations configuration in YAML format. Used by Kubernetes resource
- `api_client_id` (String) The API client ID for a resource. Used by Azure resource.
- `api_client_secret` (String, Sensitive) The API client secret for a resource. Used by Azure resource.
- `api_key` (String, Sensitive) The API key
- `api_secret` (String, Sensitive) API secret paired with `api_key`. Used by the confluent resource.
- `api_server` (String) The url for Kubernetes API server. Used by Kubernetes resource
- `api_token` (String, Sensitive) The API token for the service
- `app_id` (String, Deprecated) The app ID
- `app_key` (String, Sensitive) The application key. Used by Datadog (dd_app_key on the wire).
- `application_id` (String) The Azure application ID. Used by Azure resource.
- `application_name` (String) The application name for the service
- `arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `auth_mode` (String) Legacy MySQL/Postgres authentication mode: password or aws_rds_iam. An explicitly configured use_rds_iam switch takes precedence; omit it to retain auth-mode-only configurations.
- `automation_mode` (String) Browser automation mode for Chrome and the eleven supported managed network-device integrations.
- `aws_access_key_id` (String, Sensitive) Static AWS access key ID for Keyspaces, ElastiCache, Postgres/MySQL RDS IAM, or Kafka MSK IAM. AWS CLI uses access_key_id instead.
- `aws_arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `aws_region` (String) AWS region for Postgres/MySQL RDS IAM or Kafka MSK IAM. Empty allows supported backend hostname/broker inference. ElastiCache and Keyspaces use aws_region_name.
- `aws_region_name` (String) Keyspaces AWS region, or the AWS region of an AWS Secrets Manager secret. AWS CLI uses region_name instead.
- `aws_role_arn` (String) Role ARN for AWS CLI, Keyspaces, Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. Omit to preserve observed ownership; explicit empty clears.
- `aws_secret_access_key` (String, Sensitive) Static AWS secret access key for Keyspaces, ElastiCache, Postgres/MySQL RDS IAM, or Kafka MSK IAM. AWS CLI uses secret_access_key instead.
- `aws_service_account` (String) Kubernetes ServiceAccount identity for Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. Backend identity inference applies when use_irsa is omitted.
- `bootstrap_servers` (String) Comma-separated Kafka broker host:port string for MSK IAM. Empty falls back to bootstrap.servers in client_configuration.
- `cache_name` (String) ElastiCache replication group or serverless cache identifier used for IAM token signing. Empty allows backend hostname inference.
- `cache_type` (String) ElastiCache IAM signing resource type: serverless or replication_group. Empty allows backend hostname inference.
- `client_cert` (String, Sensitive) Client certificate presented to the server. Used by MySQL and ProxySQL.
- `client_certificate` (String, Sensitive) Client certificate bundle. Used by MongoDB, mongodb36, and syslog.
- `client_configuration` (String, Sensitive) Full client configuration (properties format). Used by the kafka resource.
- `client_id` (String) The client ID of a OAuth application. Used by Google, Okta resource
- `client_key` (String, Sensitive) Client TLS private key. Used by MySQL, ProxySQL, and syslog (with client_certificate).
- `client_secret` (String, Sensitive) The client secret for a resource. Used by Azure, Google, Okta resources.
- `clientcert` (String, Sensitive) The Snowflake client certificate. Used by Snowflake resource
- `cluster_cert` (String, Sensitive) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `cluster_token` (String, Sensitive) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `connect_server_url` (String) URL of the 1Password Connect server. Only used when `use_connect_server` is set.
- `cpu` (String) CPU request for the remote desktop pod. Used by the adaptiveremotedesktop resource.
- `create_if_not_exists` (Boolean) AWS CLI/Keyspaces: create the Kubernetes service account if missing (NOT the keyspace). Omit to preserve observation; explicit false disables creation.
- `credential_json` (String, Sensitive) Google service-account credentials JSON. Used by the big_query resource.
- `database_account` (String, Deprecated) The database account
- `database_name` (String) The name of the database to connect to. Used by CockroachDB, Postgres, Mysql resources
- `database_password` (String, Sensitive, Deprecated) The database password
- `database_username` (String, Deprecated) The database username
- `dd_api_key` (String, Sensitive) The Datadog API key
- `dd_site` (String) The Datadog site to send data to
- `default_cluster` (String) The default cluster
- `default_user` (String) Default user for the Services resource
- `domain` (String) The domain name for a resource. Used by Google, Okta resource
- `enable_allowed_commands` (Boolean) AWS CLI only: when enabled and both command and parameter are nonempty strings, the backend checks the requested service/subcommand against allowed_commands. If CLI parameters are absent or either value is missing, empty, or not a string, it falls back to an interactive /bin/bash shell before allowlist validation, even when enabled. This is not a command-only security boundary. Empty or whitespace-only allowed_commands means allow all, not deny all. This is separate from authorization resource scoping.
- `fields` (String, Sensitive) Form fields to fill during automated login for Chrome and managed network-device browser sessions.
- `host` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hostname` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hosts` (List of String) List of hosts. Used by Services resource
- `image` (String) The Docker image to use for the YugabyteDB resource
- `index` (String) The Elasticsearch index to send data to
- `is_redis_labs` (Boolean) The target is a Redis Enterprise Cloud (Redis Labs) endpoint. Used by the redis resource.
- `key` (String, Sensitive) The SSH key to use when connecting to the instance. If not specified, password authentication will be used. Used by SSH resource
- `key_file` (String, Sensitive) The content of GCP key file. Used by GCP resource
- `keyspace` (String) Keyspaces keyspace.
- `keyspaces_endpoint` (String) Keyspaces Cassandra endpoint hostname.
- `ldap_encryption_method` (String) LDAP encryption method (e.g. none, ssl, starttls). Used by the ldap and rdpldap resources.
- `ldap_hostname` (String) Hostname of the LDAP server. Used by the rdpldap resource.
- `ldap_port` (String) Port of the LDAP server. Used by the rdpldap resource.
- `ldap_search_bind_dn` (String) DN used to bind for user searches. Used by the ldap and rdpldap resources.
- `ldap_search_bind_password` (String, Sensitive) Password for `ldap_search_bind_dn`. Used by the ldap and rdpldap resources.
- `ldap_user_base_dn` (String) Base DN user searches start from. Used by the ldap and rdpldap resources.
- `ldap_user_name_attribute` (String) Attribute holding the username. Used by the ldap and rdpldap resources.
- `log_group_name` (String) CloudWatch log group to write to. Used by the awscloudwatchlogs resource.
- `log_stream_name` (String) CloudWatch log stream to write to. Used by the awscloudwatchlogs resource.
- `login_url` (String) The login URL for a resource
- `machine` (String) Heroku API host. Used by the heroku resource.
- `memory` (String) Memory request for the remote desktop pod. Used by the adaptiveremotedesktop resource.
- `namespace` (String) Namespace where pods will be created. Used by Kubernetes resource
- `network_id` (String) The network ID for ZeroTier network
- `node_affinity` (String) The node affinity configuration in YAML format. Used by Kubernetes resource
- `node_selector` (String) The node selector configuration in YAML format. Used by Kubernetes resource
- `old_version` (Boolean) Use the old-version client behavior for MySQL or ProxySQL.
- `organization_id` (String)
- `password` (String, Sensitive) Password for the adaptive integration authentication.Used by CockroachDB, Postgres, Mysql, SSH resources
- `port` (String) Port number of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `prestart` (String, Sensitive) Commands to run before the browser starts. Used by the chrome resource.
- `private_key` (String, Sensitive) The private key for the service
- `project_id` (String) The GCP project ID. Used by GCP resource
- `protocol` (String) The protocol to use when connecting to the resource
- `proxysql_admin_password` (String, Sensitive) Password for the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_admin_port` (String) Port of the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_admin_username` (String) Username for the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_hostgroup_id` (String) Hostgroup id queries are routed to. Used by the proxysql resource.
- `public_key` (String, Sensitive) Public key of the Atlas API key pair. Withheld by the server on read.
- `region` (String) The AWS region of the AWS Secrets Manager secret
- `region_name` (String) The AWS region name. Used by AWS resource.
- `resource` (String) Confluent resource (cluster) identifier. Used by the confluent resource.
- `role` (String) The Snowflake role name. Used by Snowflake resource
- `root_cert` (String, Sensitive) Root certificate for MySQL, ProxySQL, YugabyteDB, or CockroachDB Secrets Manager. Plain Postgres/CockroachDB use tls_root_cert.
- `schema` (String) The Snowflake schema name. Used by Snowflake resource
- `script` (String, Sensitive) Automation script to run for Chrome and managed network-device browser sessions.
- `secret_access_key` (String, Sensitive) The AWS secret access key in plaintext. Used by AWS resource.
- `secret_id` (String, Sensitive) The AWS Secrets Manager secret ID
- `service_account` (String, Sensitive) Google directory service-account JSON credential, 1Password service-account token, or AWS CLI/Keyspaces Kubernetes service-account name. Sensitive values remain in Terraform state. Directory backend redaction is not guaranteed for every representation.
- `service_account_name` (String) The service account name to use for the YugabyteDB resource
- `service_name` (String) Oracle service name. Used by the oracle resource.
- `shared_secret` (String, Sensitive) The shared secret for the integration
- `ssl_mode` (String) SSL mode for MySQL, Postgres, CockroachDB, Redshift, ProxySQL, SQL Server, and Azure SQL Server. SQL Server/Azure SQL additionally support false and skip-verify; MySQL/ProxySQL accept only verify_identity, verify_ca, required, preferred, disabled, the legacy CLI prefix require (preserved literally), or empty (omit the CLI SSL-mode flag). PostgreSQL spellings such as verify-full are not MySQL/ProxySQL modes. Use required rather than legacy require for ProxySQL CLI export/apply compatibility. Absent MySQL mode defaults to require for compatibility; absent ProxySQL mode remains empty. ClickHouse uses use_tls.
- `storage` (String) Persistent storage size for the remote desktop pod. Used by the adaptiveremotedesktop resource.
- `sub_system_name` (String) The sub system name for the service
- `tags` (List of String) Optional tags
- `targets` (Block List) List of RDP targets. Used by the adaptive_rdp resource. Each block is one Windows host with its own credentials. Rejected on any other resource type. (see [below for nested schema](#nestedblock--targets))
- `tenant_id` (String) The Azure tenant ID. Used by Azure resource.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `tls_ca_cert` (String) CA certificate used to verify the server. Used by Redis and syslog.
- `tls_cert_file` (String, Sensitive) The certificate file to use for the Postgres-like resources.
- `tls_enabled` (Boolean) Connect over TLS. Used by Redis, ElastiCache, DocumentDB (including Secrets Manager), syslog, and services.
- `tls_key_file` (String, Sensitive) The key file to use for the Postgres-like resources.
- `tls_root_cert` (String, Sensitive) The root certificate to use for the Postgres-like resources.
- `tls_skip_verify` (Boolean) Skip verification of the server certificate. Used by Redis, ElastiCache, and syslog.
- `token` (String, Sensitive) API token. Used by the heroku resource.
- `token_id` (String, Sensitive) The token ID for the service
- `tolerations` (String) The tolerations configuration in YAML format. Used by Kubernetes resource
- `uri` (String, Sensitive) Connection string to a resource. Used by MongoDB
- `url` (String) The URL for the service
- `urls` (String) Comma-separated list of URLs. Used by Services resource
- `use_connect_server` (Boolean) Reach 1Password through a self-hosted Connect server instead of the service account API.
- `use_iam_auth` (Boolean) Use AWS IAM authentication for ElastiCache. Omit to preserve observation; explicitly set false to disable.
- `use_irsa` (Boolean) Select the AWS identity source for Postgres/MySQL RDS IAM, Kafka MSK IAM, or ElastiCache IAM. False selects static keys. Omission preserves backend inference. Kafka sessions require a role or service account, not a bare platform identity.
- `use_msk_iam` (Boolean) Use AWS MSK IAM authentication for Kafka. Omitted switches retain backend nil/inference; explicit false disables IAM.
- `use_proxy` (Boolean) Whether to use proxy
- `use_rds_iam` (Boolean) Authenticate Postgres/MySQL with short-lived AWS RDS IAM tokens. Enable IAM on RDS and provision the database user for the target engine. Omit to preserve backend auth-mode inference; explicit false overrides legacy auth_mode. TLS cannot be disabled.
- `use_role_arn` (Boolean) Use AWS role authentication for AWS CLI or Keyspaces. Role authentication takes precedence when use_service_account is also true.
- `use_service_account` (Boolean) Whether to use service account authentication. Used by AWS CLI and Keyspaces; omit to preserve observation and explicitly set false to disable.
- `use_tenant` (Boolean) Whether to use tenant for Azure Active Directory authentication
- `use_tls` (Boolean) Connect over TLS. Used by ClickHouse, MongoDB, and mongodb36. Omission preserves observed TLS on refresh/import; configure false explicitly to disable it. ClickHouse does not support ssl_mode.
- `username` (String) Username for the adaptive resource authentication. Used by CockroachDB, Postgres, Mysql, SSH resources
- `value` (String, Sensitive) The value to encrypt. Used by the adaptiveencrypt resource.
- `version` (String) Configuration version for Keyspaces or MongoDB. Other integration versions are provider-owned envelope constants.
- `warehouse` (String) The Snowflake warehouse name. Used by Snowflake resource
- `webhook_url` (String, Sensitive) Incoming webhook URL. Used by the msteams_workflow and slack_webhook resources.
- `webui_port` (String) The web UI port

### Read-Only

- `applied_digests` (Map of String) Opaque server fingerprints of the write-only secret arguments as of the last write by this provider, used to detect out-of-band secret changes on refresh. Not comparable across resources or workspaces.
- `id` (String) The ID of this resource.
- `known_empty_secret_keys` (List of String) Server configuration keys of scalar sensitive strings explicitly observed empty. Retained until a verified provider write reconciles a subsequent credential addition; absent keys remain unknown.

<a id="nestedblock--targets"></a>
### Nested Schema for `targets`

Required:

- `host` (String) Hostname or IP of the Windows server.
- `id` (String) Unique identifier for the target within the fleet.
- `password` (String, Sensitive) Password to authenticate with the target.
- `username` (String) Username to authenticate with the target.

Optional:

- `domain` (String) Optional Windows domain for the target.
- `name` (String) Human-friendly name shown in the in-browser target picker.
- `port` (Number) RDP port. Defaults to 3389.
- `record` (Boolean) Per-target session-recording override. If unset, inherits the global recording setting (COLLECT_RDP_RECORDINGS).


<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String)
- `delete` (String)
- `update` (String)

## Import

Resources can be imported using the resource ID:

```shell
terraform import adaptive_resource.example resource-id
```

Import brings back everything except the credentials the platform withholds, and warns naming the arguments it could not populate. Write those into your configuration before the next `terraform apply` — an update sends the whole configuration, so a credential left empty is cleared on the resource.
