package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVariableCRUD(t *testing.T) {
	value := "first"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/terraform/variable/create", "/api/v1/terraform/variable/update/var-1":
			var request VariableRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
			}
			value = request.Value
			_ = json.NewEncoder(w).Encode(VariableResponse{
				ID: "var-1", Name: request.Name, Value: request.Value, Labels: request.Labels,
			})
		case "/api/v1/terraform/variable/read/var-1":
			_ = json.NewEncoder(w).Encode(VariableResponse{
				ID: "var-1", Name: "database_host", Value: value, Labels: []string{"production"},
			})
		case "/api/v1/terraform/variable/delete/var-1":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("token", server.URL)
	ctx := context.Background()
	created, err := client.CreateVariable(ctx, VariableRequest{
		Name: "database_host", Value: "first", Labels: []string{"production"},
	})
	if err != nil || created == nil || created.ID != "var-1" {
		t.Fatalf("create got (%+v, %v)", created, err)
	}
	updated, err := client.UpdateVariable(ctx, "var-1", VariableRequest{
		Name: "database_host", Value: "second", Labels: []string{"production"},
	})
	if err != nil || updated.Value != "second" {
		t.Fatalf("update got (%+v, %v)", updated, err)
	}
	read, err := client.GetVariable(ctx, "var-1")
	if err != nil || read.Value != "second" {
		t.Fatalf("read got (%+v, %v)", read, err)
	}
	if err := client.DeleteVariable(ctx, "var-1"); err != nil {
		t.Fatal(err)
	}
}

func TestVariableReadAndDeleteAreIdempotentWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := NewClient("token", server.URL)

	read, err := client.GetVariable(context.Background(), "missing")
	if err != nil || read != nil {
		t.Fatalf("read missing got (%+v, %v), want nil, nil", read, err)
	}
	if err := client.DeleteVariable(context.Background(), "missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}
