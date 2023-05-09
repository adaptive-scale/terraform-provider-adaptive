# Adding A Resource To Adaptive

Adaptive's terraform provider helps automate creation, deletion and updation of different resources in your Adaptive.live workspace.

## Add a PostgreSQL database

---

The following terraform file will add a [postgres database](https://docs.adaptive.live/integrations/postgres) in your Adaptive workspace. For more information about other types of resources, as well as their argument spec, see the [adaptive_resource](/docs/resources/resource.md) page.

## Example Usage

```hcl

# Secrets
variable "username" {
    type = string
}
variable "password" {
    type = string
}

resource "adaptive_resource" "postgres_test" {
    type          = "postgres"
    name          = "postgres-test"
    host          = "example.com"
    port          = "5432"
    username      = var.username
    password      = var.password
    database_name = "postgresadmin"
}
```

After applying and creating the resource

```bash
$ terraform init
$ terraform apply
```

You will then see your resource _"postgres-test"_ in your Workspace's [Resources tab](https://app.adaptive.live/resources)
