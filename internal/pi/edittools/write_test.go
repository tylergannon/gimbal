package edittools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "write-test.txt")

	res, err := WriteTool(dir, nil).Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"content": "Test content",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, res); got != "Successfully wrote to "+path {
		t.Fatalf("result = %q", got)
	}
	if res.Details != nil {
		t.Fatalf("write details must be nil, got %#v", res.Details)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Test content" {
		t.Fatalf("file = %q", data)
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "test.txt")

	res, err := WriteTool(dir, nil).Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"content": "Nested content",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, res); !strings.Contains(got, "Successfully wrote") {
		t.Fatalf("result = %q", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Nested content" {
		t.Fatalf("file = %q", data)
	}
}

func TestWriteDefinitionCarriesPromptContribution(t *testing.T) {
	definition := WriteDefinition("", nil)
	if definition.PromptSnippet != WritePromptSnippet {
		t.Fatalf("snippet = %q", definition.PromptSnippet)
	}
	if _, ok := definition.Parameters.Properties["content"]; !ok {
		t.Fatal("write schema missing content")
	}
}
