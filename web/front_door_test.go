package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/host"
)

// The desktop window opens at /window, into the project it last showed; `/`
// stays the projects home, on the desktop as on the web.
func TestDesktopFrontDoorReopensTheLastProject(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, dir := range []string{a, b} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	instance, err := NewInstance(t.Context(), filepath.Join(base, "instance"), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.cancel(nil); instance.Wait() })
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(path string, desktop bool) (int, string, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, "http://"+instance.Address()+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if desktop {
			request.Header.Set("X-Gimbal-Desktop", "macos")
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		return response.StatusCode, response.Header.Get("Location"), string(body)
	}

	if status, location, _ := get("/window", true); status != http.StatusTemporaryRedirect || location != "/" {
		t.Fatalf("desktop window with no projects: %d %q, want the projects home", status, location)
	}
	if status, _, body := get("/", true); status != http.StatusOK || !strings.Contains(body, "Open a project folder") {
		t.Fatalf("desktop home with no projects: %d, empty state missing", status)
	}
	first, err := instance.Owner.AdmitProject(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := instance.Owner.AdmitProject(b)
	if err != nil {
		t.Fatal(err)
	}
	if status, location, _ := get("/window", true); status != http.StatusTemporaryRedirect || location != "/projects/"+first.ID()+"/recordings" {
		t.Fatalf("desktop window with nothing remembered: %d %q", status, location)
	}
	chat := "/projects/" + second.ID() + "/conversations/c1"
	if err := instance.Owner.RememberWindow(host.WindowState{Project: second.ID(), Path: chat}); err != nil {
		t.Fatal(err)
	}
	if status, location, _ := get("/window", true); status != http.StatusTemporaryRedirect || location != chat {
		t.Fatalf("desktop window with a remembered place: %d %q", status, location)
	}
	if status, location, _ := get("/window", false); status != http.StatusTemporaryRedirect || location != "/" {
		t.Fatalf("web window: %d %q, want the projects home", status, location)
	}
	for _, desktop := range []bool{true, false} {
		if status, _, body := get("/", desktop); status != http.StatusOK || !strings.Contains(body, first.Path()) {
			t.Fatalf("projects home (desktop %v): %d, project list missing", desktop, status)
		}
	}
}
