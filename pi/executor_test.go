package pi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutorForkHistorySurvivesParentClose(t *testing.T) {
	var turns int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveRouterModels(w, r) {
			return
		}
		raw, _ := io.ReadAll(r.Body)
		turns++
		if turns == 2 && !strings.Contains(string(raw), "REMEMBER611") {
			t.Error("restored fork lost prior conversation")
		}
		writeFakeCompletion(w, "answer", 1)
	}))
	defer server.Close()
	cfg := Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }}
	a := newAdapter(cfg)
	parent, err := a.CreateSession(t.Context(), "diffusion/deepseek-4.1-flash", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.RunTurn(t.Context(), parent, "REMEMBER611", nil, nil); err != nil {
		t.Fatal(err)
	}
	child, err := a.Fork(t.Context(), parent)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "native.json")
	if err = a.SaveSession(t.Context(), child, path); err != nil {
		t.Fatal(err)
	}
	if err = a.DetachSession(t.Context(), child); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(t.Context(), parent); err != nil {
		t.Fatal(err)
	}
	restored := newAdapter(cfg)
	id, err := restored.LoadSession(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restored.RunTurn(t.Context(), id, "continue", nil, nil); err != nil {
		t.Fatal(err)
	}
	state, err := restored.session(id)
	if err != nil {
		t.Fatal(err)
	}
	root := state.root
	if err = restored.Close(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("closed session root remains: %v", err)
	}
}
func TestExecutorCanCloseSessionBeforeFirstNativeTurn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !serveRouterModels(w, r) {
			t.Error("unexpected model call")
		}
	}))
	defer server.Close()
	cfg := Config{Endpoint: server.URL + "/v1", APIKey: func() string { return "fake-router-key" }}
	a := newAdapter(cfg)
	id, err := a.CreateSession(t.Context(), "diffusion/deepseek-4.1-flash", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state, _ := a.session(id)
	root := state.root
	path := filepath.Join(t.TempDir(), "native.json")
	if err = a.SaveSession(t.Context(), id, path); err != nil {
		t.Fatal(err)
	}
	if err = a.DetachSession(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	restored := newAdapter(cfg)
	id, err = restored.LoadSession(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = restored.Close(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("closed session root remains: %v", err)
	}
}
