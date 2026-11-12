package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestServicesAuthorizationResourceTypes(t *testing.T) {
	validator := ResourceAdaptiveAuthorization().Schema["resource_type"].ValidateDiagFunc
	for _, resourceType := range []string{
		"servicelist", "kubernetes", "mongodb", "mongodb_atlas", "mongodb_aws_secrets_manager", "mongo36",
		"elasticsearch", "ssh", "postgres", "mysql", "sql_server", "sqlserver_aws_secrets_manager",
		"postgres_aws_secrets_manager", "mysql_aws_secrets_manager", "yugabytedb", "cockroachdb", "proxysql",
	} {
		t.Run(resourceType, func(t *testing.T) {
			if diagnostics := validator(resourceType, nil); diagnostics.HasError() {
				t.Fatalf("valid type rejected: %+v", diagnostics)
			}
		})
	}
	for _, invalid := range []interface{}{"", "unknown", 1} {
		if !validator(invalid, nil).HasError() {
			t.Errorf("invalid type accepted: %v", invalid)
		}
	}
}

func TestServicesAuthorizationProviderCRUDCompatibility(t *testing.T) {
	for _, resourceType := range []string{"servicelist", "postgres", "ssh"} {
		t.Run(resourceType, func(t *testing.T) {
			const initial = "allow:\n  - operation_type: GET\n    route: /api/*\n"
			const updated = "deny:\n  - operation_type: DELETE\n    route: /api/*\n"
			policy := initial
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "test-token" {
					t.Error("service token was not passed through")
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/terraform/authorization/create", "/api/v1/terraform/authorization/update/auth-1":
					if r.Method != http.MethodPost {
						t.Errorf("write method = %s", r.Method)
					}
					key, wantPolicy := "resource", initial
					if r.URL.Path != "/api/v1/terraform/authorization/create" {
						key, wantPolicy = "resourceType", updated
					}
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					want := map[string]string{"name": "services-read", "description": "policy", key: resourceType, "permissions": wantPolicy}
					if !reflect.DeepEqual(body, want) {
						t.Errorf("wire payload = %#v, want %#v", body, want)
					}
					policy = body["permissions"]
					if key == "resource" {
						_, _ = w.Write([]byte(`{"id":"auth-1"}`))
					} else {
						_, _ = w.Write([]byte(`{"ok":true}`))
					}
				case "/api/v1/terraform/authorization/read/auth-1":
					// The server retains these legacy aliases for released providers.
					_ = json.NewEncoder(w).Encode(map[string]string{"id": "auth-1", "name": "services-read", "description": "policy", "resource_type": resourceType, "permissions": policy, "Status": "created"})
				case "/api/v1/terraform/authorization/delete/auth-1":
					deleted = true
					_, _ = w.Write([]byte(`{"ok":true}`))
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := adaptive.NewClient("test-token", server.URL)
			res := ResourceAdaptiveAuthorization()
			data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
				"name": "services-read", "description": "policy", "resource_type": resourceType, "permissions": initial,
			})
			ctx := context.Background()
			if diagnostics := res.CreateContext(ctx, data, client); diagnostics.HasError() {
				t.Fatalf("create: %+v", diagnostics)
			}
			if data.Id() != "auth-1" {
				t.Fatalf("create ID = %q", data.Id())
			}
			if err := data.Set("permissions", updated); err != nil {
				t.Fatal(err)
			}
			if diagnostics := res.UpdateContext(ctx, data, client); diagnostics.HasError() {
				t.Fatalf("update: %+v", diagnostics)
			}
			if diagnostics := res.ReadContext(ctx, data, client); diagnostics.HasError() {
				t.Fatalf("read: %+v", diagnostics)
			}
			if data.Get("permissions") != updated || data.Get("resource_type") != resourceType {
				t.Fatalf("read did not preserve type and policy: %+v", data.State())
			}
			if diagnostics := res.DeleteContext(ctx, data, client); diagnostics.HasError() {
				t.Fatalf("delete: %+v", diagnostics)
			}
			if !deleted || data.Id() != "" {
				t.Fatal("delete did not remove the authorization from state")
			}
		})
	}
}
