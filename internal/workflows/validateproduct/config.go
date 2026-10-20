package validateproduct

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tylergannon/gimbal"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Suite describes the product and up to three independent user workloads.
// Paths are relative to the suite file, not the observing project's directory.
type Suite struct {
	Product       string     `json:"product" yaml:"product"`
	Guides        []string   `json:"guides" yaml:"guides"`
	Workloads     []Workload `json:"workloads" yaml:"workloads"`
	OutputDir     string     `json:"output_dir" yaml:"output_dir"`
	PlaywrightCLI string     `json:"playwright_cli" yaml:"playwright_cli"`
	IssueRepo     string     `json:"issue_repo" yaml:"issue_repo"`
}

type Workload struct {
	Name           string `json:"name" yaml:"name"`
	AssignmentFile string `json:"assignment_file" yaml:"assignment_file"`
	Workdir        string `json:"workdir" yaml:"workdir"`
	Start          string `json:"start" yaml:"start"`
	Ready          string `json:"ready" yaml:"ready"`
	URL            string `json:"url" yaml:"url"`
}

func readSuite(ctx context.Context, name string) (Suite, error) {
	var suite Suite
	code, body, stderr, err := gimbal.RunCommand(ctx, "read-suite", "/", "cat", "--", name)
	data := []byte(body)
	if code != 0 {
		return suite, fmt.Errorf("read suite: exit %d: %s", code, stderr)
	}
	if err != nil {
		return suite, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&suite); err != nil {
		return suite, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return suite, fmt.Errorf("expected one JSON or YAML document")
	}
	if strings.TrimSpace(suite.Product) == "" || suite.OutputDir == "" || len(suite.Workloads) < 1 || len(suite.Workloads) > 3 {
		return suite, fmt.Errorf("product, output_dir, and one to three workloads are required")
	}
	base := filepath.Dir(name)
	suite.OutputDir = absolute(base, suite.OutputDir)
	if suite.PlaywrightCLI == "" {
		suite.PlaywrightCLI = "playwright-cli"
	}
	if strings.ContainsRune(suite.PlaywrightCLI, filepath.Separator) {
		suite.PlaywrightCLI = absolute(base, suite.PlaywrightCLI)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(suite.IssueRepo) {
		return suite, fmt.Errorf("issue_repo is required as owner/repository")
	}
	files := make([]string, 0, len(suite.Guides)+len(suite.Workloads))
	for i, path := range suite.Guides {
		suite.Guides[i] = absolute(base, path)
		files = append(files, suite.Guides[i])
	}
	names, dirs := map[string]bool{}, []string{}
	for i := range suite.Workloads {
		w := &suite.Workloads[i]
		if strings.TrimSpace(w.Name) == "" || names[w.Name] || w.AssignmentFile == "" || w.Workdir == "" {
			return suite, fmt.Errorf("workloads need unique names, assignment_file, and workdir")
		}
		names[w.Name] = true
		w.AssignmentFile = absolute(base, w.AssignmentFile)
		files = append(files, w.AssignmentFile)
		w.Workdir = absolute(base, w.Workdir)
		code, _, _, err := gimbal.RunCommand(ctx, "check-workspace", "/", "test", "-d", w.Workdir)
		if err != nil || code != 0 {
			return suite, fmt.Errorf("workload %s needs an existing workspace", w.Name)
		}
		code, realDir, stderr, err := gimbal.RunCommand(ctx, "resolve-workspace", "/", "python3", "-c", "import os,sys; print(os.path.realpath(sys.argv[1]))", w.Workdir)
		if code != 0 {
			return suite, fmt.Errorf("resolve workspace: %s", stderr)
		}
		realDir = strings.TrimSpace(realDir)
		if err != nil {
			return suite, err
		}
		for _, other := range dirs {
			a, _ := filepath.Rel(other, realDir)
			b, _ := filepath.Rel(realDir, other)
			if filepath.IsLocal(a) || filepath.IsLocal(b) {
				return suite, fmt.Errorf("workload workspaces must not overlap")
			}
		}
		dirs = append(dirs, realDir)
		u, err := url.Parse(w.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return suite, fmt.Errorf("workload %s needs an http(s) url", w.Name)
		}
		if w.Start != "" && strings.TrimSpace(w.Ready) == "" {
			return suite, fmt.Errorf("workload %s startup needs a readiness command", w.Name)
		}
	}
	for _, path := range files {
		code, _, _, err := gimbal.RunCommand(ctx, "check-input", "/", "test", "-f", path)
		if err != nil || code != 0 {
			return suite, fmt.Errorf("input must be a local file: %s", path)
		}
	}
	return suite, nil
}
func absolute(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
