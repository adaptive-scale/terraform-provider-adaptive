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
| `windows` | Windows server (RDP) |
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

### Windows (RDP)

```terraform
resource "adaptive_resource" "windows" {
  name     = "windows-server"
  type     = "windows"
  hostname = "windows.example.com"
  port     = "3389"
  username = "Administrator"
  password = var.windows_password
  tags     = ["infrastructure", "windows"]
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

- `access_key_id` (String) The AWS access key id. Used by AWS resource.
- `annotations` (String) The annotations configuration in YAML format. Used by Kubernetes resource
- `api_client_id` (String) The API client ID for a resource. Used by Azure resource.
- `api_client_secret` (String) The API client secret for a resource. Used by Azure resource.
- `api_key` (String) The API key
- `api_server` (String) The url for Kubernetes API server. Used by Kubernetes resource
- `api_token` (String) The API token for the service
- `app_id` (String) The app ID
- `app_key` (String) The app key
- `application_id` (String) The Azure application ID. Used by Azure resource.
- `application_name` (String) The application name for the service
- `arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `aws_arn` (String) The ARN of the AWS IAM role to assume to access AWS Secrets Manager secret
- `aws_region_name` (String) The AWS region of the AWS Secrets Manager secret
- `client_id` (String) The client ID of a OAuth application. Used by Google, Okta resource
- `client_secret` (String) The client secret for a resource. Used by Azure, Google, Okta resources.
- `clientcert` (String) The Snowflake client certificate. Used by Snowflake resource
- `cluster_cert` (String) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `cluster_token` (String) The cluster token for Kubernetes API server. Used by Kubernetes resource
- `create_if_not_exists` (Boolean) Whether to create the Keyspaces keyspace if it does not exist
- `database_account` (String) The database account
- `database_name` (String) The name of the database to connect to. Used by CockroachDB, Postgres, Mysql resources
- `database_password` (String) The database password
- `database_username` (String) The database username
- `dd_api_key` (String) The Datadog API key
- `dd_site` (String) The Datadog site to send data to
- `default_user` (String) Default user for the Services resource
- `domain` (String) The domain name for a resource. Used by Google, Okta resource
- `host` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hostname` (String) Hostname of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `hosts` (List of String) List of hosts. Used by Services resource
- `image` (String) The Docker image to use for the YugabyteDB resource
- `index` (String) The Elasticsearch index to send data to
- `key` (String) The SSH key to use when connecting to the instance. If not specified, password authentication will be used. Used by SSH resource
- `key_file` (String) The content of GCP key file. Used by GCP resource
- `login_url` (String) The login URL for a resource
- `namespace` (String) Namespace where pods will be created. Used by Kubernetes resource
- `network_id` (String) The network ID for ZeroTier network
- `node_affinity` (String) The node affinity configuration in YAML format. Used by Kubernetes resource
- `node_selector` (String) The node selector configuration in YAML format. Used by Kubernetes resource
- `organization_id` (String)
- `password` (String) Password for the adaptive integration authentication.Used by CockroachDB, Postgres, Mysql, SSH resources
- `port` (String) Port number of the adaptive resource. Used by CockroachDB, Postgres, Mysql, SSH resources
- `private_key` (String) The private key for the service
- `project_id` (String) The GCP project ID. Used by GCP resource
- `protocol` (String) The protocol to use when connecting to the resource
- `public_key` (String)
- `region` (String) The AWS region of the AWS Secrets Manager secret
- `region_name` (String) The AWS region name. Used by AWS resource.
- `role` (String) The Snowflake role name. Used by Snowflake resource
- `root_cert` (String) The root certificate to use for the CockroachDB instance.
- `schema` (String) The Snowflake schema name. Used by Snowflake resource
- `secret_access_key` (String) The AWS secret access key in plaintext. Used by AWS resource.
- `secret_id` (String) The AWS Secrets Manager secret ID
- `service_account_name` (String) The service account name to use for the YugabyteDB resource
- `shared_secret` (String) The shared secret for the integration
- `ssl_mode` (String) The SSL mode to use when connecting to the database. Used by CockroachDB, Postgres, Mysql resources
- `sub_system_name` (String) The sub system name for the service
- `tags` (List of String) Optional tags
- `tenant_id` (String) The Azure tenant ID. Used by Azure resource.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `tls_cert_file` (String) The certificate file to use for the Postgres-like resources.
- `tls_key_file` (String) The key file to use for the Postgres-like resources.
- `tls_root_cert` (String) The root certificate to use for the Postgres-like resources.
- `token_id` (String) The token ID for the service
- `tolerations` (String) The tolerations configuration in YAML format. Used by Kubernetes resource
- `uri` (String) Connection string to a resource. Used by MongoDB
- `url` (String) The URL for the service
- `urls` (String) Comma-separated list of URLs. Used by Services resource
- `use_proxy` (Boolean) Whether to use proxy
- `use_service_account` (Boolean) Whether to use service account for authentication. Used by GCP resource
- `username` (String) Username for the adaptive resource authentication. Used by CockroachDB, Postgres, Mysql, SSH resources
- `version` (String) The version
- `warehouse` (String) The Snowflake warehouse name. Used by Snowflake resource
- `webui_port` (String) The web UI port

### Read-Only

- `id` (String) The ID of this resource.

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
