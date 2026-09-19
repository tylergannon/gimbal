package validateproduct

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Suite is the JSON/YAML description of a product and its observable features.
// Relative filesystem paths resolve from the suite file's directory.
type Suite struct {
	Product   Product   `json:"product" yaml:"product"`
	Tools     Tools     `json:"tools" yaml:"tools"`
	OutputDir string    `json:"output_dir" yaml:"output_dir"`
	Timeout   string    `json:"timeout" yaml:"timeout"`
	Features  []Feature `json:"features" yaml:"features"`
}

type Product struct {
	Name       string `json:"name" yaml:"name"`
	Workdir    string `json:"workdir" yaml:"workdir"`
	Prepare    string `json:"prepare" yaml:"prepare"`
	Start      string `json:"start" yaml:"start"`
	Ready      string `json:"ready" yaml:"ready"`
	BrowserURL string `json:"browser_url" yaml:"browser_url"`
	CLI        string `json:"cli" yaml:"cli"`
	Revision   string `json:"revision" yaml:"revision"`
}

// Tools names installed executables; bare names are resolved on PATH.
type Tools struct {
	PlaywrightCLI  string `json:"playwright_cli" yaml:"playwright_cli"`
	TerminalServer string `json:"terminal_server" yaml:"terminal_server"`
}

type Feature struct {
	ID       string `json:"id" yaml:"id"`
	Surface  string `json:"surface" yaml:"surface"`
	Setup    string `json:"setup" yaml:"setup"`
	Exercise string `json:"exercise" yaml:"exercise"`
	Expected string `json:"expected" yaml:"expected"`
}

func readSuite(name string) (Suite, time.Duration, error) {
	var suite Suite
	data, err := os.ReadFile(name)
	if err != nil {
		return suite, 0, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data)) // JSON is a YAML subset.
	decoder.KnownFields(true)
	if err := decoder.Decode(&suite); err != nil {
		return suite, 0, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return suite, 0, fmt.Errorf("expected one JSON or YAML document")
	}
	if strings.TrimSpace(suite.Product.Name) == "" || suite.Product.Workdir == "" || suite.OutputDir == "" || len(suite.Features) == 0 {
		return suite, 0, fmt.Errorf("product name, workdir, output_dir, and at least one feature are required")
	}
	base := filepath.Dir(name)
	suite.Product.Workdir = absolute(base, suite.Product.Workdir)
	suite.OutputDir = absolute(base, suite.OutputDir)
	if info, err := os.Stat(suite.Product.Workdir); err != nil || !info.IsDir() {
		return suite, 0, fmt.Errorf("product workdir must be an existing directory")
	}
	if suite.Product.Start != "" && strings.TrimSpace(suite.Product.Ready) == "" {
		return suite, 0, fmt.Errorf("a startup command requires a readiness command")
	}
	if suite.Timeout == "" {
		suite.Timeout = "15m"
	}
	timeout, err := time.ParseDuration(suite.Timeout)
	if err != nil || timeout <= 0 {
		return suite, 0, fmt.Errorf("timeout must be a positive duration")
	}
	if suite.Tools.PlaywrightCLI == "" {
		suite.Tools.PlaywrightCLI = "playwright-cli"
	}
	if suite.Tools.TerminalServer == "" {
		suite.Tools.TerminalServer = "gotty"
	}
	for _, value := range []*string{&suite.Tools.PlaywrightCLI, &suite.Tools.TerminalServer, &suite.Product.CLI} {
		if strings.ContainsRune(*value, filepath.Separator) {
			*value = absolute(base, *value)
		}
	}
	seen := map[string]bool{}
	for _, f := range suite.Features {
		if strings.TrimSpace(f.ID) == "" || seen[f.ID] || strings.TrimSpace(f.Exercise) == "" || strings.TrimSpace(f.Expected) == "" {
			return suite, 0, fmt.Errorf("features need unique nonempty IDs, exercise, and expected outcomes")
		}
		seen[f.ID] = true
		switch f.Surface {
		case "browser":
			u, e := url.Parse(suite.Product.BrowserURL)
			if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return suite, 0, fmt.Errorf("browser features require an http(s) browser_url")
			}
		case "cli":
			if suite.Product.CLI == "" {
				return suite, 0, fmt.Errorf("CLI features require product.cli")
			}
		default:
			return suite, 0, fmt.Errorf("feature %s: surface must be browser or cli", f.ID)
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

// Resolve symlinks before checking containment, including links in parent directories.
func evidencePath(dir, path string) (string, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute(dir, path))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("evidence is outside this feature: %s", path)
	}
	return resolved, nil
}
