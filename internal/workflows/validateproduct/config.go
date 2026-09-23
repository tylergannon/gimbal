package validateproduct

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Suite describes the product and up to three independent user workloads.
// Paths are relative to the suite file, not the observing project's directory.
type Suite struct {
	Product       string     `json:"product" yaml:"product"`
	Guides        []string   `json:"guides" yaml:"guides"`
	Workloads     []Workload `json:"workloads" yaml:"workloads"`
	OutputDir     string     `json:"output_dir" yaml:"output_dir"`
	Timeout       string     `json:"timeout" yaml:"timeout"`
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

func readSuite(name string) (Suite, time.Duration, error) {
	var suite Suite
	data, err := os.ReadFile(name)
	if err != nil {
		return suite, 0, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&suite); err != nil {
		return suite, 0, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return suite, 0, fmt.Errorf("expected one JSON or YAML document")
	}
	if strings.TrimSpace(suite.Product) == "" || suite.OutputDir == "" || len(suite.Workloads) < 1 || len(suite.Workloads) > 3 {
		return suite, 0, fmt.Errorf("product, output_dir, and one to three workloads are required")
	}
	base := filepath.Dir(name)
	suite.OutputDir = absolute(base, suite.OutputDir)
	if suite.PlaywrightCLI == "" {
		suite.PlaywrightCLI = "playwright-cli"
	}
	if strings.ContainsRune(suite.PlaywrightCLI, filepath.Separator) {
		suite.PlaywrightCLI = absolute(base, suite.PlaywrightCLI)
	}
	if suite.Timeout == "" {
		suite.Timeout = "1h"
	}
	timeout, err := time.ParseDuration(suite.Timeout)
	if err != nil || timeout <= 0 {
		return suite, 0, fmt.Errorf("timeout must be a positive duration")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(suite.IssueRepo) {
		return suite, 0, fmt.Errorf("issue_repo is required as owner/repository")
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
			return suite, 0, fmt.Errorf("workloads need unique names, assignment_file, and workdir")
		}
		names[w.Name] = true
		w.AssignmentFile = absolute(base, w.AssignmentFile)
		files = append(files, w.AssignmentFile)
		w.Workdir = absolute(base, w.Workdir)
		info, err := os.Stat(w.Workdir)
		if err != nil || !info.IsDir() {
			return suite, 0, fmt.Errorf("workload %s needs an existing workspace", w.Name)
		}
		realDir, err := filepath.EvalSymlinks(w.Workdir)
		if err != nil {
			return suite, 0, err
		}
		for _, other := range dirs {
			a, _ := filepath.Rel(other, realDir)
			b, _ := filepath.Rel(realDir, other)
			if filepath.IsLocal(a) || filepath.IsLocal(b) {
				return suite, 0, fmt.Errorf("workload workspaces must not overlap")
			}
		}
		dirs = append(dirs, realDir)
		u, err := url.Parse(w.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return suite, 0, fmt.Errorf("workload %s needs an http(s) url", w.Name)
		}
		if w.Start != "" && strings.TrimSpace(w.Ready) == "" {
			return suite, 0, fmt.Errorf("workload %s startup needs a readiness command", w.Name)
		}
	}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return suite, 0, fmt.Errorf("input must be a local file: %s", path)
		}
	}
	return suite, timeout, nil
}
func absolute(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
