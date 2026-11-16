package provider_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Uses isolated local httptest contracts, never a live workspace.
func TestReorderSemanticsTerraformEngine(t *testing.T) {
	if os.Getenv("ADAPTIVE_ENGINE_TEST") != "1" {
		t.Skip("requires ADAPTIVE_ENGINE_TEST=1")
	}
	tf, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	bin := os.Getenv("ADAPTIVE_ENGINE_PROVIDER_BINARY")
	if bin == "" {
		bin = filepath.Join(t.TempDir(), "terraform-provider-adaptive")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, engineSafe(string(out)))
		}
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"group", "dataprotection"} {
		t.Run(family, func(t *testing.T) {
			h, api := newFamiliesEngine(t, tf, bin, family)
			config := func(values map[string]any) {
				data := map[string]any{
					"terraform": map[string]any{"required_providers": map[string]any{"adaptive": map[string]any{"source": "adaptive-scale/adaptive"}}},
					"provider":  map[string]any{"adaptive": map[string]any{"service_token": engineToken, "workspace_url": h.url}},
					"resource":  map[string]any{"adaptive_" + family: map[string]any{"test": values}},
				}
				encoded, err := json.Marshal(data)
				if err != nil {
					t.Fatal(err)
				}
				engineWrite(t, filepath.Join(h.dir, "main.tf.json"), string(encoded))
			}
			base := func(reverse bool) map[string]any {
				names := []any{"a@example.invalid", "b@example.invalid"}
				endpoints := []any{"db-a", "db-b"}
				if reverse {
					names[0], names[1] = names[1], names[0]
					endpoints[0], endpoints[1] = endpoints[1], endpoints[0]
				}
				if family == "group" {
					return map[string]any{"name": "reorder-group", "members": names, "endpoints": endpoints}
				}
				cols := []any{map[string]any{"column_name": "email", "masking_type": "hash"}, map[string]any{"column_name": "ssn", "masking_type": "redact"}}
				if reverse {
					cols[0], cols[1] = cols[1], cols[0]
				}
				return map[string]any{"resource": "db", "scoped": true, "masks": []any{map[string]any{"database_name": "app", "tables": []any{map[string]any{"table_name": "people", "schema": "public", "masked": true, "masked_columns": cols}}}}}
			}
			config(base(false))
			h.run(0, "apply", "-auto-approve")
			before := api.mutationCount()
			config(base(true))
			for i := 0; i < 2; i++ {
				h.run(0, "plan", "-detailed-exitcode")
				h.run(0, "apply", "-refresh-only", "-auto-approve")
			}
			h.run(0, "apply", "-auto-approve")
			if api.mutationCount() != before {
				t.Fatal("reorder caused API mutation")
			}
			mutations := []string{"add", "remove", "replace", "clear", "omit"}
			if family == "dataprotection" {
				mutations = append(mutations, "mask", "scoped", "schema", "masked")
			}
			for _, change := range mutations {
				t.Run(change, func(t *testing.T) {
					values := base(true)
					if family == "group" {
						for _, field := range []string{"members", "endpoints"} {
							switch change {
							case "add":
								values[field] = append(values[field].([]any), "extra")
							case "remove":
								values[field] = values[field].([]any)[:1]
							case "replace":
								values[field].([]any)[0] = "other"
							case "clear":
								values[field] = []any{}
							case "omit":
								delete(values, field)
							}
						}
					} else {
						table := values["masks"].([]any)[0].(map[string]any)["tables"].([]any)[0].(map[string]any)
						cols := table["masked_columns"].([]any)
						switch change {
						case "add":
							table["masked_columns"] = append(cols, map[string]any{"column_name": "phone", "masking_type": "phone"})
						case "remove":
							table["masked_columns"] = cols[:1]
						case "replace":
							cols[0].(map[string]any)["column_name"] = "other"
						case "clear":
							values["masks"] = []any{}
						case "omit":
							delete(values, "masks")
						case "mask":
							cols[0].(map[string]any)["masking_type"] = "hash"
						case "scoped":
							values["scoped"] = false
						case "schema":
							table["schema"] = "private"
						case "masked":
							table["masked"] = false
						}
					}
					config(values)
					h.run(2, "plan", "-detailed-exitcode", "-out=change.tfplan")
					familiesEngineActions(h, "adaptive_"+family+".test", "change.tfplan", "update")
					if api.mutationCount() != before {
						t.Fatal("plan mutated API")
					}
				})
			}
			config(base(true))
			h.run(0, "plan", "-detailed-exitcode")
			t.Log("pure reorder: exit 0, no API mutations; substantive changes: exit 2 update plans")
		})
	}
}
