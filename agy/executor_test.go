package agy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutorCheckpointResumesNativeConversation(t *testing.T) {
	a, record := testAdapter(t)
	id, err := a.CreateSession(t.Context(), "gemini-test-low", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.RunTurn(t.Context(), id, "first", nil, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "native.json")
	if err = a.SaveSession(t.Context(), id, path); err != nil {
		t.Fatal(err)
	}
	if err = a.DetachSession(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	restored := newAdapter(a.config)
	loaded, err := restored.LoadSession(t.Context(), path, false)
	if err != nil || loaded != id {
		t.Fatalf("%q %v", loaded, err)
	}
	if _, err = restored.RunTurn(t.Context(), loaded, "second", nil, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "conversation-test") {
		t.Fatalf("did not resume native conversation: %s", raw)
	}
}
