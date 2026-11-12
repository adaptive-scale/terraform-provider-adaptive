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

func TestDataProtectionLifecycle(t *testing.T) {
	var policy adaptive.DataProtectionRequest
	var paths []string
	failDelete := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/terraform/dataprotection/create", "/api/v1/terraform/dataprotection/update/resource-id":
			if r.Method != http.MethodPost {
				t.Errorf("write used %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Error(err)
			}
		case "/api/v1/terraform/dataprotection/read/resource-id":
			if r.Method != http.MethodGet {
				t.Errorf("read used %s", r.Method)
			}
		case "/api/v1/terraform/dataprotection/delete/resource-id":
			if r.Method != http.MethodPost {
				t.Errorf("delete used %s", r.Method)
			}
			if failDelete {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`"could not turn off masking"`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"resource-id"}`))
			return
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(adaptive.DataProtectionResponse{DataProtectionRequest: policy, ID: "resource-id", AuthorizationName: "masked_db", Status: "applied"})
	}))
	defer srv.Close()
	r := ResourceAdaptiveDataProtection()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"resource": "db",
		"masks": []interface{}{map[string]interface{}{
			"database_name": "app",
			"tables": []interface{}{map[string]interface{}{
				"table_name": "users", "schema": "public", "masked": true,
				"masked_columns": []interface{}{map[string]interface{}{"column_name": "email", "masking_type": "email"}},
			}},
		}},
	})
	want := dataProtectionFromSchema(d)
	client := adaptive.NewClient("token", srv.URL)
	ctx := context.Background()
	if diags := r.CreateContext(ctx, d, client); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Id() != "resource-id" || !policy.Scoped || !reflect.DeepEqual(policy, *want) {
		t.Fatalf("create did not round trip: %#v", policy)
	}
	if d.Get("authorization_name") != "masked_db" {
		t.Fatal("missing generated authorization")
	}
	policy.Scoped = false
	policy.Masks[0].Tables[0].MaskedColumns[0].MaskingType = "redact"
	if diags := r.ReadContext(ctx, d, client); diags.HasError() {
		t.Fatal(diags)
	}
	if got := dataProtectionFromSchema(d); !reflect.DeepEqual(*got, policy) {
		t.Fatalf("read did not refresh drift: %#v", got)
	}
	if err := d.Set("masks", []interface{}{}); err != nil {
		t.Fatal(err)
	}
	if diags := r.UpdateContext(ctx, d, client); diags.HasError() {
		t.Fatal(diags)
	}
	if policy.Scoped || policy.Masks == nil || len(policy.Masks) != 0 {
		t.Fatalf("explicit false/empty policy not sent: %#v", policy)
	}
	if diags := r.DeleteContext(ctx, d, client); !diags.HasError() || d.Id() == "" {
		t.Fatal("failed disable removed state")
	}
	failDelete = false
	if diags := r.DeleteContext(ctx, d, client); diags.HasError() || d.Id() != "" {
		t.Fatalf("delete failed: %v", diags)
	}
	if len(paths) != 5 {
		t.Fatalf("unexpected requests: %v", paths)
	}
	if !r.Schema["resource"].ForceNew {
		t.Fatal("repointing a policy must replace it")
	}
}
