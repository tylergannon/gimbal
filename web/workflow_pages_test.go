package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/builtin"
)

// Exercise the shipped Go loads and SSR bundle together. A frontend build alone
// can pass while a dependency fails when the installed engine renders a page.
func TestShippedWorkflowPages(t *testing.T) {
	dist, err := fs.Sub(Build, "build")
	if err != nil {
		t.Fatal(err)
	}
	handler, mode, err := NewHandler(dist, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if mode != "prod" {
		t.Fatalf("expected shipped application, got %s", mode)
	}
	for _, workflow := range builtin.Workflows {
		for _, panel := range []string{"workflow", "guide", "run"} {
			t.Run(workflow.Name+"/"+panel, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workflows/"+workflow.Name+"?v=1&panel="+panel, nil))
				if response.Code != http.StatusOK {
					t.Fatalf("status %d: %s", response.Code, response.Body.String()[:min(response.Body.Len(), 1200)])
				}
				for _, text := range []string{workflow.Name, "Workflow views"} {
					if !strings.Contains(response.Body.String(), text) {
						t.Fatalf("page missing %q", text)
					}
				}
				if panel == "run" && !strings.Contains(response.Body.String(), "Start workflow") {
					t.Fatal("missing visible start form")
				}
			})
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workflows/missing-workflow", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown workflow status: %d", response.Code)
	}
}
