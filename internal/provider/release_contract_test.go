package provider

import (
	"encoding/csv"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"gopkg.in/yaml.v2"
)

func TestParityInventoryMatchesProvider(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "docs", "parity-completion-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		IntegrationTypes []string `json:"integration_types"`
		Counts           struct {
			Registered int `json:"registered_integrations"`
			Rows       int `json:"current_top_level_wire_rows"`
			Deltas     int `json:"original_delta_occurrences"`
			Leaves     int `json:"rdp_nested_leaves"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.IntegrationTypes) != report.Counts.Registered {
		t.Fatal("registered integration inventory count mismatch")
	}
	for _, source := range []struct{ path, symbol string }{
		{filepath.Join("components", "resource.go"), "validIntegrationTypes"},
		{filepath.Join("components", "resource.go"), "schemaToResourceIntegrationConfiguration"},
		{filepath.Join("integrations", "apply_config.go"), "applyIntegrationMapping"},
	} {
		if got := releaseIntegrationTypes(t, source.path, source.symbol); !reflect.DeepEqual(got, report.IntegrationTypes) {
			t.Fatalf("production %s differs from published integration inventory", source.symbol)
		}
	}
	for _, fixture := range []struct {
		name  string
		count int
	}{{"parity-current-integration-fields.csv", report.Counts.Rows}, {"parity-field-dispositions.csv", report.Counts.Deltas}, {"parity-rdp-target-fields.csv", report.Counts.Leaves}} {
		file, err := os.Open(filepath.Join(root, "docs", fixture.name))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(file).ReadAll()
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows)-1 != fixture.count {
			t.Fatalf("%s inventory count mismatch", fixture.name)
		}
		seen := map[string]bool{}
		for _, row := range rows[1:] {
			key := row[0] + "\x00" + row[1]
			if seen[key] {
				t.Fatalf("duplicate inventory key in %s", fixture.name)
			}
			seen[key] = true
			if fixture.name == "parity-current-integration-fields.csv" {
				attr, _ := integrations.AttrForConfigKey(integrations.WireType(row[0]), row[1])
				if attr != row[4] {
					t.Fatalf("stale reverse inventory for %s/%s", row[0], row[1])
				}
			}
		}
	}
}

// Read the actual registry and dispatch declarations rather than maintaining a
// second permissive hand-written list in the test.
func releaseIntegrationTypes(t *testing.T, path, symbol string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var node ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		switch value := n.(type) {
		case *ast.FuncDecl:
			if value.Name.Name == symbol {
				node = value.Body
			}
		case *ast.ValueSpec:
			if len(value.Names) == 1 && value.Names[0].Name == symbol && len(value.Values) == 1 {
				node = value.Values[0]
			}
		}
		return true
	})
	if node == nil {
		t.Fatalf("production declaration %s not found", symbol)
	}
	seen := map[string]bool{}
	add := func(n ast.Expr) {
		literal, ok := n.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		seen[value] = true
	}
	if values, ok := node.(*ast.CompositeLit); ok {
		for _, value := range values.Elts {
			add(value)
		}
	} else {
		ast.Inspect(node, func(n ast.Node) bool {
			if clause, ok := n.(*ast.CaseClause); ok {
				for _, value := range clause.List {
					add(value)
				}
			}
			return true
		})
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func TestReleaseWorkflowExercisesRealCLIContracts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "test.yml"))
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Run  string            `yaml:"run"`
		With map[string]string `yaml:"with"`
		Env  map[string]string `yaml:"env"`
	}
	var config struct {
		Jobs map[string]struct {
			Steps    []step `yaml:"steps"`
			Strategy struct {
				Matrix struct {
					Terraform []string `yaml:"terraform"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	engine := config.Jobs["engine"]
	if !reflect.DeepEqual(engine.Strategy.Matrix.Terraform, []string{"1.11.4", "1.14.3"}) {
		t.Fatal("real CLI matrix must exercise the declared pinned Terraform versions")
	}
	found, selected := false, false
	for _, s := range engine.Steps {
		selected = selected || s.With["terraform_version"] == "${{ matrix.terraform }}"
		if strings.Contains(s.Run, "TerraformEngine") {
			if s.Env["ADAPTIVE_ENGINE_TEST"] != "1" {
				t.Fatal("CI installs Terraform but does not opt into CLI execution")
			}
			for _, suite := range []string{"Secret", "Mappings", "Endpoint", "Cloud", "IntegrationFields", "Families", "CoreLifecycle"} {
				if !strings.Contains(s.Run, suite) {
					t.Fatalf("CI omits %s lifecycle suite", suite)
				}
			}
			found = true
		}
	}
	if !found || !selected {
		t.Fatal("real CLI matrix configuration incomplete")
	}
	generated := false
	for _, s := range config.Jobs["generate"].Steps {
		generated = generated || s.Run == "make docs"
	}
	if !generated {
		t.Fatal("documentation job must regenerate docs, not a no-op go generate")
	}
}
