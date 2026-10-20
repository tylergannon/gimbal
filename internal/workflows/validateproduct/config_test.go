package validateproduct

import (
	"context"
	"encoding/json"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/runstore"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func suiteFixture(t *testing.T, n int) (Suite, string) {
	t.Helper()
	dir := t.TempDir()
	s := Suite{Product: "Example A", OutputDir: filepath.Join(dir, "output"), IssueRepo: "example/product-a"}
	for i := range n {
		workdir := filepath.Join(dir, string(rune('a'+i)))
		if err := os.Mkdir(workdir, 0700); err != nil {
			t.Fatal(err)
		}
		assignment := filepath.Join(workdir, "assignment.md")
		if err := os.WriteFile(assignment, []byte("Use A to complete a useful task in B."), 0600); err != nil {
			t.Fatal(err)
		}
		s.Workloads = append(s.Workloads, Workload{Name: string(rune('a' + i)), AssignmentFile: assignment, Workdir: workdir, URL: "http://127.0.0.1:1234"})
	}
	return s, filepath.Join(dir, "suite.json")
}
func saveSuite(t *testing.T, s Suite, path string) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSuiteInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Suite)
		want   string
	}{
		{"valid", func(*Suite) {}, ""},
		{"too many", func(s *Suite) { s.Workloads = append(s.Workloads, s.Workloads...) }, "one to three"},
		{"shared workspace", func(s *Suite) { s.Workloads[1].Workdir = s.Workloads[0].Workdir }, "overlap"},
		{"missing local issue", func(s *Suite) { s.Workloads[0].AssignmentFile = "missing.md" }, "local file"},
		{"startup needs readiness", func(s *Suite) { s.Workloads[0].Start = "server" }, "readiness"},
		{"missing repository", func(s *Suite) { s.IssueRepo = "" }, "issue_repo is required"},
		{"bad repository", func(s *Suite) { s.IssueRepo = "https://github.com/example/repo" }, "owner/repository"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, path := suiteFixture(t, 2)
			tc.change(&s)
			saveSuite(t, s, path)
			got, err := testReadSuite(t, path)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error %v, want %s", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.PlaywrightCLI != "playwright-cli" {
				t.Fatalf("defaults: %+v", got)
			}
		})
	}
}
func TestYAMLRelativeFiles(t *testing.T) {
	s, path := suiteFixture(t, 1)
	data := "product: Example\nissue_repo: example/product-a\noutput_dir: output\nworkloads:\n  - name: first\n    assignment_file: a/assignment.md\n    workdir: a\n    url: http://localhost:1234\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := testReadSuite(t, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workloads[0].AssignmentFile != s.Workloads[0].AssignmentFile {
		t.Fatal(got.Workloads[0])
	}
	if err := os.WriteFile(path, []byte(data+"typo: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := testReadSuite(t, path); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func testReadSuite(t *testing.T, path string) (suite Suite, err error) {
	t.Helper()
	err = gimbal.Run(gimbal.Project(t.Context(), t.TempDir()), string(runstore.NewRunID()), "read-suite-test", nil, func(ctx context.Context) error {
		var readErr error
		suite, readErr = readSuite(ctx, path)
		return readErr
	})
	return
}
