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

## Secret Arguments

Credential arguments (`password`, `key`, `api_token`, `client_secret`, `secret_id`, the LDAP and ProxySQL passwords, and the rest) are marked sensitive, so Terraform keeps them out of plan output. The platform never returns them on read either. Instead it reports an opaque fingerprint per withheld argument in the computed `redacted_digests` map, which the provider compares on refresh: if a credential is changed outside Terraform, the matching argument shows up in the next plan as being re-applied from your configuration.

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
- `access_key_id` (String) The AWS access key id. Used by AWS resource.
- `annotations` (String) The annotations configuration in YAML format. Used by Kubernetes resource
- `api_client_id` (String) The API client ID for a resource. Used by Azure resource.
- `api_client_secret` (String, Sensitive) The API client secret for a resource. Used by Azure resource.
- `api_key` (String, Sensitive) The API key
- `api_secret` (String, Sensitive) API secret paired with `api_key`. Used by the confluent resource.
- `api_server` (String) The url for Kubernetes API server. Used by Kubernetes resource
- `api_token` (String, Sensitive) The API token for the service
- `app_id` (String) The app ID
- `app_key` (String, Sensitive) The app key
- `application_id` (String) The Azure application ID. Used by Azure resource.
- `application_name` (String) The application name for the service
- `arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `automation_mode` (String) Browser automation mode. Used by the chrome resource.
- `aws_access_key_id` (String) Static AWS access key id used to mint RDS IAM tokens. Requires `use_irsa = false`.
- `aws_arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `aws_region` (String) The AWS region used to mint RDS IAM tokens. Leave empty to auto-detect from an *.rds.amazonaws.com hostname. Only used when `use_rds_iam` is set.
- `aws_region_name` (String) The AWS region of the AWS Secrets Manager secret
- `aws_role_arn` (String) An IAM role to assume when minting RDS IAM tokens. With IRSA it overrides the service account's eks.amazonaws.com/role-arn annotation. Only used when `use_rds_iam` is set.
- `aws_secret_access_key` (String, Sensitive) Static AWS secret access key used to mint RDS IAM tokens. Requires `use_irsa = false`.
- `aws_service_account` (String) Name of a Kubernetes ServiceAccount in the session cluster annotated with eks.amazonaws.com/role-arn; the platform mints RDS IAM tokens on its behalf. Requires `use_irsa = true`.
- `client_cert` (String, Sensitive) Client certificate presented to the server. Used by the proxysql resource.
- `client_certificate` (String, Sensitive) Client certificate bundle. Used by the mongodb36 resource.
- `client_configuration` (String, Sensitive) Full client configuration (properties format). Used by the kafka resource.
- `client_id` (String) The client ID of a OAuth application. Used by Google, Okta resource
- `client_key` (String, Sensitive) Private key for `client_cert`. Used by the proxysql resource.
- `client_secret` (String, Sensitive) The client secret for a resource. Used by Azure, Google, Okta resources.
- `clientcert` (String, Sensitive) The Snowflake client certificate. Used by Snowflake resource
- `cluster_cert` (String) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `cluster_token` (String, Sensitive) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `connect_server_url` (String) URL of the 1Password Connect server. Only used when `use_connect_server` is set.
- `cpu` (String) CPU request for the remote desktop pod. Used by the adaptiveremotedesktop resource.
- `create_if_not_exists` (Boolean) Whether to create the Keyspaces keyspace if it does not exist
- `credential_json` (String, Sensitive) Google service-account credentials JSON. Used by the big_query resource.
- `database_account` (String) The database account
- `database_name` (String) The name of the database to connect to. Used by CockroachDB, Postgres, Mysql resources
- `database_password` (String, Sensitive) The database password
- `database_username` (String) The database username
- `dd_api_key` (String, Sensitive) The Datadog API key
- `dd_site` (String) The Datadog site to send data to
- `default_cluster` (String) The default cluster
- `default_user` (String) Default user for the Services resource
- `domain` (String) The domain name for a resource. Used by Google, Okta resource
- `fields` (String) Form fields to fill during automated login. Used by the chrome resource.
- `host` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hostname` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hosts` (List of String) List of hosts. Used by Services resource
- `image` (String) The Docker image to use for the YugabyteDB resource
- `index` (String) The Elasticsearch index to send data to
- `is_redis_labs` (Boolean) The target is a Redis Enterprise Cloud (Redis Labs) endpoint. Used by the redis resource.
- `key` (String, Sensitive) The SSH key to use when connecting to the instance. If not specified, password authentication will be used. Used by SSH resource
- `key_file` (String, Sensitive) The content of GCP key file. Used by GCP resource
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
- `old_version` (Boolean) Target a pre-2.x ProxySQL admin interface. Used by the proxysql resource.
- `organization_id` (String)
- `password` (String, Sensitive) Password for the adaptive integration authentication.Used by CockroachDB, Postgres, Mysql, SSH resources
- `port` (String) Port number of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `prestart` (String) Commands to run before the browser starts. Used by the chrome resource.
- `private_key` (String, Sensitive) The private key for the service
- `project_id` (String) The GCP project ID. Used by GCP resource
- `protocol` (String) The protocol to use when connecting to the resource
- `proxysql_admin_password` (String, Sensitive) Password for the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_admin_port` (String) Port of the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_admin_username` (String) Username for the ProxySQL admin interface. Used by the proxysql resource.
- `proxysql_hostgroup_id` (String) Hostgroup id queries are routed to. Used by the proxysql resource.
- `public_key` (String)
- `region` (String) The AWS region of the AWS Secrets Manager secret
- `region_name` (String) The AWS region name. Used by AWS resource.
- `resource` (String) Confluent resource (cluster) identifier. Used by the confluent resource.
- `role` (String) The Snowflake role name. Used by Snowflake resource
- `root_cert` (String) The root certificate to use for the CockroachDB instance.
- `schema` (String) The Snowflake schema name. Used by Snowflake resource
- `script` (String) Automation script to run in the browser. Used by the chrome resource.
- `secret_access_key` (String, Sensitive) The AWS secret access key in plaintext. Used by AWS resource.
- `secret_id` (String, Sensitive) The AWS Secrets Manager secret ID
- `service_account` (String) 1Password service account token.
- `service_account_name` (String) The service account name to use for the YugabyteDB resource
- `service_name` (String) Oracle service name. Used by the oracle resource.
- `shared_secret` (String, Sensitive) The shared secret for the integration
- `ssl_mode` (String) The SSL mode to use when connecting to the database. Used by CockroachDB, Postgres, Mysql resources
- `storage` (String) Persistent storage size for the remote desktop pod. Used by the adaptiveremotedesktop resource.
- `sub_system_name` (String) The sub system name for the service
- `tags` (List of String) Optional tags
- `targets` (Block List) List of RDP targets. Used by the adaptive_rdp resource. Each block is one Windows host with its own credentials. Rejected on any other resource type. (see [below for nested schema](#nestedblock--targets))
- `tenant_id` (String) The Azure tenant ID. Used by Azure resource.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `tls_ca_cert` (String) CA certificate used to verify the server. Used by the redis resource.
- `tls_cert_file` (String) The certificate file to use for the Postgres-like resources.
- `tls_enabled` (Boolean) Connect over TLS. Used by the redis, awselasticcache and awsdocumentdb_aws_secret_manager resources.
- `tls_key_file` (String, Sensitive) The key file to use for the Postgres-like resources.
- `tls_root_cert` (String) The root certificate to use for the Postgres-like resources.
- `tls_skip_verify` (Boolean) Skip verification of the server certificate. Used by the redis and awselasticcache resources.
- `token` (String, Sensitive) API token. Used by the heroku resource.
- `token_id` (String, Sensitive) The token ID for the service
- `tolerations` (String) The tolerations configuration in YAML format. Used by Kubernetes resource
- `uri` (String, Sensitive) Connection string to a resource. Used by MongoDB
- `url` (String) The URL for the service
- `urls` (String) Comma-separated list of URLs. Used by Services resource
- `use_connect_server` (Boolean) Reach 1Password through a self-hosted Connect server instead of the service account API.
- `use_irsa` (Boolean) Mint RDS IAM tokens with `aws_service_account`, or with the platform's own IRSA / instance-profile identity when it is empty. Set to false to use `aws_access_key_id` / `aws_secret_access_key` instead. Only used when `use_rds_iam` is set.
- `use_proxy` (Boolean) Whether to use proxy
- `use_rds_iam` (Boolean) Authenticate with short-lived AWS RDS IAM auth tokens instead of a stored password. The database user must be granted rds_iam, the RDS instance must have IAM authentication enabled, and `ssl_mode` cannot be `disable`. Used by Postgres resources.
- `use_service_account` (Boolean) Whether to use service account for authentication. Used by GCP resource
- `use_tenant` (Boolean) Whether to use tenant for Azure Active Directory authentication
- `use_tls` (Boolean) Connect over TLS. Used by the mongodb36 resource.
- `username` (String) Username for the adaptive resource authentication. Used by CockroachDB, Postgres, Mysql, SSH resources
- `value` (String) The value to encrypt. Used by the adaptiveencrypt resource.
- `version` (String) The version
- `warehouse` (String) The Snowflake warehouse name. Used by Snowflake resource
- `webhook_url` (String, Sensitive) Incoming webhook URL. Used by the msteams_workflow and slack_webhook resources.
- `webui_port` (String) The web UI port

### Read-Only

- `id` (String) The ID of this resource.
- `redacted_digests` (Map of String) Opaque server fingerprints of the write-only secret arguments, used to detect out-of-band secret changes on refresh. Not comparable across resources or workspaces.

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
