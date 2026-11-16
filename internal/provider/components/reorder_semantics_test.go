package components

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func reorderPolicy(columns ...string) map[string]interface{} {
	cols := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		cols = append(cols, map[string]interface{}{"column_name": col, "masking_type": "hash"})
	}
	return map[string]interface{}{"resource": "db", "scoped": true, "masks": []interface{}{map[string]interface{}{"database_name": "app", "tables": []interface{}{map[string]interface{}{"table_name": "people", "schema": "public", "masked": true, "masked_columns": cols}}}}}
}

func assertReorderDiff(t *testing.T, r *schema.Resource, prior, next map[string]interface{}, changed bool) {
	t.Helper()
	d := schema.TestResourceDataRaw(t, r.Schema, prior)
	d.SetId("existing")
	diff, err := r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(next), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := diff != nil && !diff.Empty()
	if got != changed {
		t.Fatalf("meaningful diff = %t, want %t: %#v", got, changed, diff)
	}
}

func TestMaskReorderSemantics(t *testing.T) {
	r := ResourceAdaptiveDataProtection()
	t.Run("column reorder", func(t *testing.T) {
		assertReorderDiff(t, r, reorderPolicy("email", "ssn"), reorderPolicy("ssn", "email"), false)
	})
	for name, next := range map[string]map[string]interface{}{
		"add":           reorderPolicy("email", "ssn", "phone"),
		"remove":        reorderPolicy("email"),
		"clear":         reorderPolicy(),
		"mask change":   reorderPolicy("ssn", "email"),
		"scope change":  reorderPolicy("ssn", "email"),
		"schema change": reorderPolicy("ssn", "email"),
		"masked change": reorderPolicy("ssn", "email"),
		"omit":          {"resource": "db", "scoped": true},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "scope change" {
				next["scoped"] = false
			}
			if name == "mask change" {
				reorderTable(next)["masked_columns"].([]interface{})[0].(map[string]interface{})["masking_type"] = "redact"
			}
			if name == "schema change" {
				reorderTable(next)["schema"] = "private"
			}
			if name == "masked change" {
				reorderTable(next)["masked"] = false
			}
			assertReorderDiff(t, r, reorderPolicy("email", "ssn"), next, true)
		})
	}
	t.Run("database and table reorder", func(t *testing.T) {
		prior, next := reorderPolicy("email", "ssn"), reorderPolicy("ssn", "email")
		for _, policy := range []map[string]interface{}{prior, next} {
			db := policy["masks"].([]interface{})[0].(map[string]interface{})
			db["tables"] = append(db["tables"].([]interface{}), map[string]interface{}{"table_name": "events", "masked": true})
			policy["masks"] = append(policy["masks"].([]interface{}), map[string]interface{}{"database_name": "audit"})
		}
		db := next["masks"].([]interface{})[0].(map[string]interface{})
		tables := db["tables"].([]interface{})
		tables[0], tables[1] = tables[1], tables[0]
		dbs := next["masks"].([]interface{})
		dbs[0], dbs[1] = dbs[1], dbs[0]
		assertReorderDiff(t, r, prior, next, false)
	})
	for _, level := range []string{"table", "database"} {
		t.Run("duplicate "+level+" precedence", func(t *testing.T) {
			prior, next := reorderPolicy("email"), reorderPolicy("ssn")
			first := prior["masks"].([]interface{})[0].(map[string]interface{})
			second := next["masks"].([]interface{})[0].(map[string]interface{})
			if level == "database" {
				prior["masks"], next["masks"] = []interface{}{first, second}, []interface{}{second, first}
			} else {
				a, b := reorderTable(prior), reorderTable(next)
				first["tables"], second["tables"] = []interface{}{a, b}, []interface{}{b, a}
			}
			assertReorderDiff(t, r, prior, next, true)
		})
	}
	t.Run("omit columns", func(t *testing.T) {
		next := reorderPolicy("email", "ssn")
		delete(reorderTable(next), "masked_columns")
		assertReorderDiff(t, r, reorderPolicy("email", "ssn"), next, true)
	})
	t.Run("omit tables", func(t *testing.T) {
		next := reorderPolicy("email", "ssn")
		delete(next["masks"].([]interface{})[0].(map[string]interface{}), "tables")
		assertReorderDiff(t, r, reorderPolicy("email", "ssn"), next, true)
	})
	t.Run("duplicate column precedence is not a reorder", func(t *testing.T) {
		prior, next := reorderPolicy("email", "email"), reorderPolicy("email", "email")
		reorderTable(prior)["masked_columns"].([]interface{})[0].(map[string]interface{})["masking_type"] = "redact"
		reorderTable(next)["masked_columns"].([]interface{})[1].(map[string]interface{})["masking_type"] = "redact"
		assertReorderDiff(t, r, prior, next, true)
	})
	t.Run("ambiguous names remain meaningful", func(t *testing.T) { assertReorderDiff(t, r, reorderPolicy("a", "b"), reorderPolicy("a=hash b"), true) })
}

func reorderTable(policy map[string]interface{}) map[string]interface{} {
	return policy["masks"].([]interface{})[0].(map[string]interface{})["tables"].([]interface{})[0].(map[string]interface{})
}

func TestReorderListStateCompatibility(t *testing.T) {
	team := ResourceAdaptiveTeam()
	policy := ResourceAdaptiveDataProtection()
	for _, r := range []*schema.Resource{team, policy} {
		if r.SchemaVersion != 0 || len(r.StateUpgraders) != 0 {
			t.Fatal("unexpected schema migration: reorder fix must preserve legacy state")
		}
	}
	for name, field := range map[string]*schema.Schema{
		"members": team.Schema["members"], "endpoints": team.Schema["endpoints"],
		"masks":          policy.Schema["masks"],
		"tables":         policy.Schema["masks"].Elem.(*schema.Resource).Schema["tables"],
		"masked_columns": policy.Schema["masks"].Elem.(*schema.Resource).Schema["tables"].Elem.(*schema.Resource).Schema["masked_columns"],
	} {
		if field.Type != schema.TypeList || !field.Optional || field.Computed {
			t.Fatalf("%s no longer has its optional non-computed list encoding", name)
		}
	}
	t.Run("empty policy omission", func(t *testing.T) {
		assertReorderDiff(t, policy, map[string]interface{}{"resource": "db", "masks": []interface{}{}}, map[string]interface{}{"resource": "db"}, false)
	})
	for _, field := range []string{"members", "endpoints"} {
		t.Run("empty "+field+" omission", func(t *testing.T) {
			assertReorderDiff(t, team, map[string]interface{}{"name": "group", field: []interface{}{}}, map[string]interface{}{"name": "group"}, false)
		})
	}
}

func TestTeamReorderSemantics(t *testing.T) {
	for _, field := range []string{"members", "endpoints"} {
		t.Run(field, func(t *testing.T) {
			prior := map[string]interface{}{"name": "group", field: []interface{}{"a", "b"}}
			for _, tc := range []struct {
				name    string
				values  []interface{}
				changed bool
			}{
				{"reorder", []interface{}{"b", "a"}, false},
				{"add", []interface{}{"b", "a", "c"}, true},
				{"remove", []interface{}{"b"}, true},
				{"replace", []interface{}{"c", "a"}, true},
				{"clear", []interface{}{}, true},
				{"omit", nil, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					next := map[string]interface{}{"name": "group"}
					if tc.values != nil {
						next[field] = tc.values
					}
					assertReorderDiff(t, ResourceAdaptiveTeam(), prior, next, tc.changed)
				})
			}
		})
	}
}
