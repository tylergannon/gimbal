package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGeneratorInputsSeparateImplementationFromObservedSource(t *testing.T) {
	t.Setenv("GOWORK", "off")
	root := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.27.2\n")
	write("internal/generate/appgen/main.go", `package main
import "fixture/metadata"
func main() { println(metadata.Defaults) }
`)
	write("metadata/defaults.go", `package metadata
import _ "embed"
//go:embed defaults.json
var Defaults string
`)
	write("metadata/defaults.json", `{"name":"literal generator default"}`)
	write("application/source.go", "package application\nconst Observed = 17\n")
	inputs, err := generatorFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"go.mod", "go.sum", "internal/generate/appgen/*.go", "metadata/*.go", "metadata/defaults.json"} {
		if !slices.Contains(inputs, filepath.Join(root, relative)) {
			t.Errorf("compiled input %s missing from %v", relative, inputs)
		}
	}
	if slices.Contains(inputs, filepath.Join(root, "application/*.go")) {
		t.Fatal("source-only application requires generator restart")
	}
}
