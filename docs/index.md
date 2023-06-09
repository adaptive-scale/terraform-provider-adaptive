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

- [aws](https://docs.adaptive.live/integrations/aws)
- [azure](https://docs.adaptive.live/integrations/azure)
- [cockroachdb](https://docs.adaptive.live/integrations/cockroachdb)
- [gcp](https://docs.adaptive.live/integrations/gcp)
- [google](https://docs.adaptive.live/integrations/google)
- [mongo](https://docs.adaptive.live/integrations/mongo)
- [mongodb atlas](https://docs.adaptive.live/integrations/mongodb_atlas)
- [aws documentdb](https://docs.adaptive.live/integrations/documentdb)
- [mysql](https://docs.adaptive.live/integrations/mysql)
- [okta](https://docs.adaptive.live/integrations/okta)
- [aws redshift](https://docs.adaptive.live/integrations/redshift)
- [postgres](https://docs.adaptive.live/integrations/postgres)
- [services](https://docs.adaptive.live/integrations/services)
- [ssh](https://docs.adaptive.live/integrations/ssh)
- [zerotier](https://docs.adaptive.live/integrations/zerotier)

## Schema

- `workspace_url` (String) The workspace to use for the provider. If not set, the default workspace will be used app.adaptive.live
