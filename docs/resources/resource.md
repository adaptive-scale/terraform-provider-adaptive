---
page_title: "adaptive_resource Resource - terraform-provider-adaptive"
subcategory: ""
description: |-
  Provide settings for Adaptive Resources
---

# adaptive_resource (Resource)

A resource is an integration provided by Adaptive to connect to another platform or service like AWS, PostgreSQL etc.

## Example Usage

```hcl
# `EOT` sequence are used to create multiline strings
resource "adaptive_resource" "k8s-test" {
  name          = "awsk8s"
  type          = "kubernetes"
  api_server    = "https://hostid.gr7.us-east-2.eks.amazonaws.com"
  cluster_token = <<EOT
multiline string of cluster-token
EOT
  cluster_cert  = <<EOT
-----BEGIN CERTIFICATE-----
multiline cert
-----END CERTIFICATE-----
EOT
}

```

## Schema

### Required

These arguments are required for all resource types:

- `name` (String) Name of the Adaptive resource
- `type` (String) Type of the Adaptive resource

### Resource specific arguments

The following arguments are supported by each individual adaptive-resource:

- `aws`
  - `name` (Required) - The name of the configuration
  - `region_name` - (Required) The AWS region name to use for AWS CLI resource
  - `access_key_id` - (Required) The AWS access key ID to use for AWS CLI resource
  - `secret_access_key` - (Required) The AWS secret access key to use for AWS CLI resource
- `azure`
  - `name` (Required) - The name of the Azure resource to create.
  - `tenant_id` - The Azure tenant ID.
  - `application_id` - The Azure application ID.
  - `client_secret` - The Azure client secret.
- `cockroachdb`
  - `name` - (Required) The name of the CockroachDB database to create.
  - `host` - (Required) The hostname of the CockroachDB instance to connect to.
  - `port` - (Required) The port number of the CockroachDB instance to connect to.
  - `username` - (Required) The username to authenticate with the CockroachDB instance.
  - `password` - (Required) The password to authenticate with the CockroachDB instance.
  <!-- - `ssl_mode` - (Optional) The SSL mode to use when connecting to the CockroachDB instance. Defaults to "verify-full". -->
  - `database_name` - (Optional) The name of the CockroachDB database to create. If not specified, the default database will be used.
  - `root_cert` - (Optional) The root certificate to use for the CockroachDB instance.
- `gcp`
  - `name` - (Required) The name of the GCP instance to create.
  - `project_id` - (Required) The GCP project ID.
  - `key_file` - (Required) The path to the GCP service account key file.
- `google`
  - `name` - (Required) The name of the Google OAuth resource to create.
  - `domain` - (Optional) A domain to restrict the Google OAuth resource to. Defaults to https://accounts.google.com.
  - `client_id` - (Required) The client ID for the Google OAuth resource.
  - `client_secret` - (Required) The client secret for the Google OAuth resource.
- `documentdb`
  - `name` (Required) - The name of the AWS Documentdb resource to create.
  - `uri` - (Required) MongoDB Connection URI
- `mongodb`
  - `name` (Required) - The name of the Mongodb resource to create.
  - `uri` - (Required) MongoDB Connection URI
- `mysql`
  - `name` - (Required) The name of the MySQL database to create.
  - `database_name` - (Optional) The name of the MySQL database to create. If not specified, the default database will be used.
  - `host` - (Required) The hostname of the MySQL instance to connect to.
  - `port` - (Required) The port number of the MySQL instance to connect to.
  - `username` - (Required) The username to authenticate with the MySQL instance.
  - `password` - (Required) The password to authenticate with the MySQL instance.
- `okta`
  - `name` - (Required) The name of the Okta OAuth resource to create.
  - `domain` - (Required) The Okta domain to use for authentication.
  - `client_id` - (Required) The client ID of the Okta OAuth application.
  - `client_secret` - (Required) The client secret of the Okta OAuth application.
- `postgres`
  - `name` - (Required) The name of the Postgres database to create.
  - `host` - (Required) The hostname of the Postgres instance to connect to.
  - `port` - (Required) The port number of the Postgres instance to connect to.
  - `username` - (Required) The username to authenticate with the Postgres instance.
  - `password` - (Required) The password to authenticate with the Postgres instance.
  <!-- - `ssl_mode` - (Required) The SSL mode to use when connecting to the Postgres instance. -->
  - `database_name` - (Optional) The name of the Postgres database to create. If not specified, the default database will be used.
- `ssh`
  - `name` - (Required) The name of the SSH instance to create.
  - `username` - (Required) The username to authenticate with the SSH instance.
  - `host` - (Required) The hostname of the SSH instance to connect to.
  - `port` - (Required) The port number of the SSH instance to connect to.
  - `password` - (Optional) The password to use when connecting to the SSH instance. If not specified, the default value is an empty string.
  - `key` - (Optional) The SSH key to use when connecting to the instance. If not specified, password authentication will be used.

### Read-Only

- `id` (String) A unique identifer for the Adaptive resource.
