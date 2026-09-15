package components

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adaptive "github.com/adaptive-scale/terraform-provider-adaptive/internal/terraform-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestScriptHelpTextLifecycle(t *testing.T) {
	const instructions = "Choose a tenant.\nExample: tenant = café."
	stored := ""
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/read/") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "sc-1", "name": "backup", "endpoint": "prod-db", "helpText": stored, "commandOmitted": true})
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		value, ok := payload["helpText"].(string)
		if !ok {
			t.Error("write must include helpText, even when empty")
		}
		stored = value
		writes++
		_ = json.NewEncoder(w).Encode(map[string]string{"ID": "sc-1"})
	}))
	defer srv.Close()
	client := adaptive.NewClient("test-token", srv.URL)
	res := ResourceAdaptiveScript()
	d := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
		"name": "backup", "command": "pg_dump {{tenant}}", "endpoint": "prod-db", "help_text": instructions,
	})
	if diags := res.CreateContext(context.Background(), d, client); diags.HasError() {
		t.Fatal(diags)
	}
	if d.Id() != "sc-1" || stored != instructions {
		t.Fatalf("create did not persist help text: id=%q text=%q", d.Id(), stored)
	}

	// Import starts with no metadata and must hydrate help text from the read API.
	imported := schema.TestResourceDataRaw(t, res.Schema, nil)
	imported.SetId("sc-1")
	if diags := res.ReadContext(context.Background(), imported, client); diags.HasError() {
		t.Fatal(diags)
	}
	if got := imported.Get("help_text"); got != instructions {
		t.Fatalf("import help_text = %q", got)
	}

	// Build an update with an unchanged endpoint, as required by the resource.
	config := map[string]interface{}{"name": "backup", "command": "pg_dump {{tenant}}", "endpoint": "prod-db", "help_text": instructions}
	old := schema.TestResourceDataRaw(t, res.Schema, config)
	old.SetId("sc-1")
	delete(config, "help_text")
	diff, err := schema.InternalMap(res.Schema).Diff(context.Background(), old.State(), terraform.NewResourceConfigRaw(config), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := schema.InternalMap(res.Schema).Data(old.State(), diff)
	if err != nil {
		t.Fatal(err)
	}
	if diags := res.UpdateContext(context.Background(), updated, client); diags.HasError() {
		t.Fatal(diags)
	}
	if stored != "" || writes != 2 {
		t.Fatalf("update did not clear help text: %q, writes=%d", stored, writes)
	}
	if diags := res.ReadContext(context.Background(), imported, client); diags.HasError() {
		t.Fatal(diags)
	}
	if got := imported.Get("help_text"); got != "" {
		t.Fatalf("refresh retained stale help text: %q", got)
	}
}
