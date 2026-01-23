# Integration Templates

This directory contains templates for creating new integrations for the Adaptive Terraform provider.

## Template Files

### 1. `integration_template.go.tmpl`
Full integration template with support for:
- Tags (`TagsFromSchema`)
- Default cluster (`DefaultClusterFromSchema`)
- Complete CRUD operations

Use this template for integrations that need tags and default cluster support.

### 2. `integration_simple.go.tmpl`
Simplified integration template with:
- Basic CRUD operations
- Empty tags (`[]string{}`)
- Empty default cluster (`""`)

Use this template for simple integrations that don't need tags or default cluster.

### 3. `config_only.go.tmpl`
Configuration-only template containing:
- Configuration struct definition
- Schema-to-configuration conversion function

Use this template when you only need the configuration struct (e.g., for integrations managed elsewhere).

## Template Variables

Replace these placeholders when creating a new integration:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.IntegrationName}}` | PascalCase name for Go types and functions | `Redis`, `Elasticsearch`, `Kafka` |
| `{{.ResourceName}}` | snake_case name for Terraform resource | `redis`, `elasticsearch`, `kafka` |
| `{{.ResourceType}}` | API resource type identifier | `redis`, `elasticsearch`, `kafka` |

## Creating a New Integration

1. Copy the appropriate template to the integrations directory:
   ```bash
   cp template/integration_template.go.tmpl ../newservice.go
   ```

2. Replace all template variables:
   ```bash
   sed -i 's/{{.IntegrationName}}/NewService/g' ../newservice.go
   sed -i 's/{{.ResourceName}}/newservice/g' ../newservice.go
   sed -i 's/{{.ResourceType}}/newservice/g' ../newservice.go
   ```

3. Add your integration-specific fields to:
   - The configuration struct (with `yaml` tags)
   - The schema definition (in `resourceAdaptive...()`)
   - The schema conversion function (`SchemaTo...()`)

4. Register the resource in the provider (usually in `provider.go`):
   ```go
   "adaptive_newservice": resourceAdaptiveNewService(),
   ```

## Field Type Examples

### String field
```go
// In struct:
Hostname string `yaml:"hostname"`

// In schema:
"hostname": {
    Type:        schema.TypeString,
    Required:    true,
    Description: "The hostname to connect to.",
},

// In conversion:
Hostname: d.Get("hostname").(string),
```

### Boolean field
```go
// In struct:
UseSSL bool `yaml:"use_ssl"`

// In schema:
"use_ssl": {
    Type:        schema.TypeBool,
    Optional:    true,
    Default:     true,
    Description: "Whether to use SSL.",
},

// In conversion:
UseSSL: d.Get("use_ssl").(bool),
```

### Optional field with default
```go
// In struct:
Port string `yaml:"port"`

// In schema:
"port": {
    Type:        schema.TypeString,
    Optional:    true,
    Default:     "6379",
    Description: "The port number.",
},

// In conversion:
Port: d.Get("port").(string),
```

### Sensitive field
```go
// In schema:
"password": {
    Type:        schema.TypeString,
    Required:    true,
    Sensitive:   true,
    Description: "The password for authentication.",
},
```
