package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dlpPolicyRequestFromSchema must map every Terraform attribute onto the API
// request the backend upsert route consumes, or the declared policy silently
// loses controls.
func TestDLPPolicyRequestFromSchema(t *testing.T) {
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":                     "pii-strict",
		"description":              "strict PII handling",
		"enabled":                  true,
		"block_screenshot":         true,
		"block_clipboard_copy":     true,
		"block_clipboard_paste":    false,
		"block_file_transfer":      true,
		"block_bulk_export":        true,
		"max_export_rows":          500,
		"block_session_log_export": true,
		"blocked_surfaces":         []interface{}{"webshell"},
		"exceptions":               []interface{}{`"sre" in teams && deviceTrusted`},
		"applies_to_labels":        []interface{}{"pii"},
		"workspace_default":        false,
	})

	req, err := dlpPolicyRequestFromSchema(d)
	if err != nil {
		t.Fatalf("dlpPolicyRequestFromSchema: %v", err)
	}

	want := &adaptive.DLPPolicyRequest{
		Name:                  "pii-strict",
		Description:           "strict PII handling",
		Enabled:               true,
		BlockScreenshot:       true,
		BlockClipboardCopy:    true,
		BlockClipboardPaste:   false,
		BlockFileTransfer:     true,
		BlockBulkExport:       true,
		MaxExportRows:         500,
		BlockSessionLogExport: true,
		BlockedSurfaces:       []string{"webshell"},
		Exceptions:            []string{`"sre" in teams && deviceTrusted`},
		AppliesToLabels:       []string{"pii"},
		WorkspaceDefault:      false,
	}
	if !reflect.DeepEqual(req, want) {
		t.Errorf("request mismatch:\n got %+v\nwant %+v", req, want)
	}
}

func TestDLPPolicyRequestFromSchema_EmptyName(t *testing.T) {
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})

	if _, err := dlpPolicyRequestFromSchema(d); err == nil {
		t.Fatal("expected an error for an empty name")
	}
}

// Read must refresh state from GET /terraform/dlppolicy/{name}, keyed off the
// resource ID (the policy name) — this is also the import-by-name path, where
// only the ID is populated.
func TestResourceAdaptiveDLPPolicyRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/terraform/dlppolicy/pii-strict" {
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "dlp-1",
			"name": "pii-strict",
			"description": "strict PII handling",
			"enabled": true,
			"blockScreenshot": true,
			"blockClipboardCopy": true,
			"blockClipboardPaste": false,
			"blockFileTransfer": true,
			"blockBulkExport": true,
			"maxExportRows": 500,
			"blockSessionLogExport": true,
			"blockedSurfaces": ["webshell"],
			"exceptions": ["\"sre\" in teams && deviceTrusted"],
			"appliesToLabels": ["pii"],
			"workspace_default": false
		}`))
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveDLPPolicy()
	// Import scenario: only the ID is known.
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})
	d.SetId("pii-strict")

	if diags := ResourceAdaptiveDLPPolicyRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}

	checks := map[string]interface{}{
		"name":                     "pii-strict",
		"description":              "strict PII handling",
		"enabled":                  true,
		"block_screenshot":         true,
		"block_clipboard_copy":     true,
		"block_clipboard_paste":    false,
		"block_file_transfer":      true,
		"block_bulk_export":        true,
		"max_export_rows":          500,
		"block_session_log_export": true,
		"workspace_default":        false,
		"policy_id":                "dlp-1",
	}
	for key, want := range checks {
		if got := d.Get(key); got != want {
			t.Errorf("%s not refreshed: got %v want %v", key, got, want)
		}
	}
	if got := d.Get("blocked_surfaces").([]interface{}); len(got) != 1 || got[0] != "webshell" {
		t.Errorf("blocked_surfaces not refreshed: got %v", got)
	}
	if got := d.Get("exceptions").([]interface{}); len(got) != 1 || got[0] != `"sre" in teams && deviceTrusted` {
		t.Errorf("exceptions not refreshed: got %v", got)
	}
	if got := d.Get("applies_to_labels").([]interface{}); len(got) != 1 || got[0] != "pii" {
		t.Errorf("applies_to_labels not refreshed: got %v", got)
	}
}

// A policy deleted out-of-band must be dropped from state (404 read) so
// Terraform plans a recreate instead of failing.
func TestResourceAdaptiveDLPPolicyRead_GoneClearsState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "gone"})
	d.SetId("gone")

	if diags := ResourceAdaptiveDLPPolicyRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read returned diagnostics: %+v", diags)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared for a deleted policy, got %q", d.Id())
	}
}

// Exception expressions are validated server-side (expr-lang); the server's
// validation message must reach the practitioner verbatim.
func TestResourceAdaptiveDLPPolicyCreate_SurfacesServerValidationError(t *testing.T) {
	const serverMsg = `invalid exception expression "\"sre\" in teams &&": unexpected token EOF (1:24)`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/terraform/dlppolicy" {
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(map[string]string{"error": serverMsg})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":       "pii-strict",
		"exceptions": []interface{}{`"sre" in teams &&`},
	})

	diags := ResourceAdaptiveDLPPolicyCreate(context.Background(), d, client)
	if !diags.HasError() {
		t.Fatal("expected a diagnostic error for an invalid exception expression")
	}
	if !strings.Contains(diags[0].Summary, serverMsg) {
		t.Errorf("server validation error not surfaced verbatim:\n got %q\nwant substring %q", diags[0].Summary, serverMsg)
	}
	if d.Id() != "" {
		t.Errorf("expected no ID set on failed create, got %q", d.Id())
	}
}

// Create must upsert via POST /terraform/dlppolicy, use the policy name as the
// resource ID, and record the server-side id.
func TestResourceAdaptiveDLPPolicyCreate(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/terraform/dlppolicy" {
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "dlp-1", "name": "pii-strict", "enabled": true}`))
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name":             "pii-strict",
		"enabled":          true,
		"block_screenshot": true,
		"max_export_rows":  500,
	})

	if diags := ResourceAdaptiveDLPPolicyCreate(context.Background(), d, client); diags.HasError() {
		t.Fatalf("create returned diagnostics: %+v", diags)
	}

	if d.Id() != "pii-strict" {
		t.Errorf("expected resource ID to be the policy name, got %q", d.Id())
	}
	if got := d.Get("policy_id").(string); got != "dlp-1" {
		t.Errorf("policy_id not recorded: got %q want %q", got, "dlp-1")
	}
	if gotBody["name"] != "pii-strict" {
		t.Errorf("upsert body name: got %v", gotBody["name"])
	}
	if gotBody["blockScreenshot"] != true {
		t.Errorf("upsert body blockScreenshot: got %v", gotBody["blockScreenshot"])
	}
	if gotBody["maxExportRows"] != float64(500) {
		t.Errorf("upsert body maxExportRows: got %v", gotBody["maxExportRows"])
	}
}

// Delete must call DELETE /terraform/dlppolicy/{name} and clear the ID.
func TestResourceAdaptiveDLPPolicyDelete(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveDLPPolicy()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "pii-strict"})
	d.SetId("pii-strict")

	if diags := ResourceAdaptiveDLPPolicyDelete(context.Background(), d, client); diags.HasError() {
		t.Fatalf("delete returned diagnostics: %+v", diags)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/terraform/dlppolicy/pii-strict" {
		t.Errorf("unexpected delete request: %s %s", gotMethod, gotPath)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}
