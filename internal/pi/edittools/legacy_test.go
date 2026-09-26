package edittools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrepareEditArgumentsKeepsLegacyFieldsOutOfSchema(t *testing.T) {
	definition := EditDefinition("", nil)
	if _, ok := definition.Parameters.Properties["oldText"]; ok {
		t.Fatal("public schema must not carry oldText")
	}
	if _, ok := definition.Parameters.Properties["newText"]; ok {
		t.Fatal("public schema must not carry newText")
	}
}

func TestPrepareEditArgumentsFoldsLegacyFields(t *testing.T) {
	prepared := PrepareEditArguments(map[string]any{
		"path":    "file.txt",
		"oldText": "before",
		"newText": "after",
	})
	want := map[string]any{
		"path": "file.txt",
		"edits": []any{
			map[string]any{"oldText": "before", "newText": "after"},
		},
	}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestPrepareEditArgumentsAppendsLegacyToExistingEdits(t *testing.T) {
	prepared := PrepareEditArguments(map[string]any{
		"path":    "file.txt",
		"edits":   []any{map[string]any{"oldText": "a", "newText": "b"}},
		"oldText": "c",
		"newText": "d",
	})
	want := map[string]any{
		"path": "file.txt",
		"edits": []any{
			map[string]any{"oldText": "a", "newText": "b"},
			map[string]any{"oldText": "c", "newText": "d"},
		},
	}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestPrepareEditArgumentsPassesValidInputThrough(t *testing.T) {
	input := map[string]any{
		"path":  "file.txt",
		"edits": []any{map[string]any{"oldText": "a", "newText": "b"}},
	}
	prepared := PrepareEditArguments(input)
	if reflect.ValueOf(prepared).Pointer() != reflect.ValueOf(input).Pointer() {
		t.Fatal("valid input should pass through by reference")
	}
}

func TestPrepareEditArgumentsFoldsSingleEditObject(t *testing.T) {
	prepared := PrepareEditArguments(map[string]any{
		"path":  "file.txt",
		"edits": map[string]any{"oldText": "a", "newText": "b"},
	})
	want := map[string]any{
		"path": "file.txt",
		"edits": []any{
			map[string]any{"oldText": "a", "newText": "b"},
		},
	}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestPrepareEditArgumentsParsesJSONStringEdits(t *testing.T) {
	prepared := PrepareEditArguments(map[string]any{
		"path":  "file.txt",
		"edits": `[{"oldText":"a","newText":"b"}]`,
	})
	want := map[string]any{
		"path": "file.txt",
		"edits": []any{
			map[string]any{"oldText": "a", "newText": "b"},
		},
	}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestPrepareEditArgumentsLeavesInvalidJSONString(t *testing.T) {
	prepared := PrepareEditArguments(map[string]any{
		"path":  "file.txt",
		"edits": "not json",
	})
	want := map[string]any{
		"path":  "file.txt",
		"edits": "not json",
	}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestPreparedLegacyArgumentsExecute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.txt")
	writeTestFile(t, path, "before\n")

	tool := EditTool(dir, nil)
	prepared := PrepareEditArguments(map[string]any{
		"path":    "legacy.txt",
		"oldText": "before",
		"newText": "after",
	})
	prepared = tool.PrepareArguments(prepared)

	res, err := tool.Execute(context.Background(), "tool-1", prepared, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Successfully replaced 1 block(s) in legacy.txt."
	if got := resultText(t, res); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "after\n" {
		t.Fatalf("file = %q", data)
	}
}

func TestEditParameterSchemaJSON(t *testing.T) {
	definition := EditDefinition("", nil)
	raw, err := json.Marshal(definition.Parameters)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := definition.Parameters.Properties["path"]; !ok {
		t.Fatalf("schema missing path: %s", raw)
	}
}
