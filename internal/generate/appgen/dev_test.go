package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupWorkflowRepairRequiresRecompiledGenerator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "polytype_gen.go")
	const old = "package workflow\nconst CodecField = \"old\"\n"
	const repaired = "package workflow\nconst CodecField = \"new\"\n"
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	generate := func() error { return os.WriteFile(path, []byte(repaired), 0644) }
	if err := regenerateCompiledWorkflow(path, generate); err == nil || !strings.Contains(err.Error(), "restart just dev") {
		t.Fatalf("changed compiled codec must stop startup: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != repaired {
		t.Fatalf("startup must retain repaired source for next compilation: %q, %v", data, err)
	}
	if err := regenerateCompiledWorkflow(path, generate); err != nil {
		t.Fatalf("unchanged codec must allow the next startup: %v", err)
	}
	want := errors.New("literal projection failure")
	if err := regenerateCompiledWorkflow(path, func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("lost generator diagnostic: %v", err)
	}
}
