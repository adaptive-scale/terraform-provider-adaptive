---
page_title: "adaptive Provider"
subcategory: ""
description: |-
---

# Adaptive Provider

You can use the Adaptive Terraform provider to configure and manage your Adaptive account and resources. This project allows you to leverage Terraform to complete the following tasks in Adaptive:

- Create and Modify Adaptive resources
- Create and Modify Adaptive endpoints
- Create and Grant authorizations to endpoints
- Manage resources, endpoints and authorizations

## Example Usage

To install the Adaptive provider, copy and paste this code into your Terraform configuration.

```hcl
# Install Adaptive Provider
terraform {
  required_providers {
    adaptive = {
      source = "adaptive-scale/adaptive"
      version = "0.0.5"
    }
  }
}

provider "adaptive" {}
```

## Configuration

To configure the provider for use, you must add [Service Token](https://docs.adaptive.live/). Generate the token using your User Settings Page.

Now you can you that token like so:

   ```bash
   # this will store the token in ~/.adaptive/token
   $ adaptive login
   ```

   ```hcl
   provider "adaptive" {}
   ```

## Features

Current list of supported adaptive resources:

- [aws](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/aws)
- [azure](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/azure)
- [cockroachdb](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/cockroachdb)
- [gcp](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/mastergcp)
- [google](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/google)
- [mongo](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/mongo)
- [mongodb atlas](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/mongodb_atlas)
- [aws documentdb](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/documentdb)
- [mysql](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/mysql)
- [okta](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/okta)
- [aws redshift](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/redshift)
- [postgres](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/postgres)
- [services](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/services)
- [ssh](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/ssh)
- [zerotier](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/zerotier)
- [mysql_aws_secrets_manager](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/mysql_aws_secrets_manager)
- [postgres_aws_secrets_manager](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/postgres_aws_secrets_manager)
- [mongodb_aws_secrets_manager](https://github.com/adaptive-scale/adaptive-terraform-examples/tree/master/mongodb_aws_secrets_manager)


## Schema

- `workspace_url` (String) The workspace to use for the provider. If not set, the default workspace will be used app.adaptive.live
