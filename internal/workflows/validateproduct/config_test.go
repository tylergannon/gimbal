package validateproduct

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
)

const validSuite = `product:
  name: Example
  workdir: .
  browser_url: http://127.0.0.1:8080
output_dir: artifacts
features:
  - id: home
    surface: browser
    exercise: Open the home page
    expected: The product name is visible
`

func TestReadSuite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "suite.yaml")
	for _, tc := range []struct{ name, input, wantError string }{
		{"yaml", validSuite, ""},
		{"json", `{"product":{"name":"Example","workdir":".","cli":"./bin/example"},"output_dir":"artifacts","features":[{"id":"help","surface":"cli","exercise":"Run help","expected":"Exit 0"}]}`, ""},
		{"unknown field", validSuite + "typo: true\n", "field typo"},
		{"multiple documents", validSuite + "---\n{}", "one JSON or YAML"},
		{"duplicate IDs", validSuite + "  - id: home\n    surface: browser\n    exercise: Again\n    expected: Name\n", "unique"},
		{"bad surface", strings.Replace(validSuite, "surface: browser", "surface: desktop", 1), "surface must"},
		{"startup without readiness", strings.Replace(validSuite, "  name: Example", "  name: Example\n  start: example", 1), "readiness"},
		{"invalid timeout", validSuite + "timeout: 0s\n", "positive duration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(file, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			suite, timeout, err := readSuite(file)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if timeout != 15*time.Minute || suite.Product.Workdir != dir || suite.OutputDir != filepath.Join(dir, "artifacts") {
				t.Fatalf("incorrect resolved inputs: %+v, %s", suite, timeout)
			}
			if tc.name == "json" && suite.Product.CLI != filepath.Join(dir, "bin/example") {
				t.Fatal(suite.Product.CLI)
			}
		})
	}
}

func TestMissingPrerequisiteRetainsEveryFeature(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "suite.yaml")
	input := validSuite + "  - id: second\n    surface: browser\n    exercise: Open settings\n    expected: Settings visible\ntools:\n  playwright_cli: ./missing-browser-driver\n"
	if err := os.WriteFile(file, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "missing-prerequisite", nil, func(ctx context.Context) error {
		return ValidateProduct(ctx, gimble.Env{WorkDir: dir}, Params{SuiteFile: file})
	})
	if err == nil || !strings.Contains(err.Error(), "browser driver prerequisite") {
		t.Fatalf("error = %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "artifacts", "validation-*", "report.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("reports = %v, %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Error == "" || len(got.Features) != 2 {
		t.Fatalf("report = %+v", got)
	}
	for _, f := range got.Features {
		if f.Status != "blocked" || f.Reason != "not exercised" {
			t.Fatalf("feature = %+v", f)
		}
	}
}

func TestGraphHasNoDiagnostics(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", Graph.Diagnostics)
	}
}
