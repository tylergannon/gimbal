package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestCentralServesNestedCommandOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := NewInstance(ctx, t.TempDir(), nil, WithPort(0))
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	owner, err := instance.Owner.AdmitProject(project)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(project, ".gimbal")
	err = gimbal.Run(gimbal.Project(owner.RequestContext(ctx), state), "output", nil, func(ctx context.Context) error {
		return gimbal.Scope(ctx, "child", func(ctx context.Context) error {
			_, _, _, err := gimbal.RunCommand(ctx, "emit", project, "printf", "actual command output")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	runs, err := os.ReadDir(filepath.Join(state, "runs"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	endpoint := "http://" + instance.address + "/projects/" + owner.ID() + "/api/runs/" + runs[0].Name() + "/commands/stdout/" + url.PathEscape("child.1/emit.1")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "actual command output" {
		t.Fatalf("HTTP %d body=%q err=%v", response.StatusCode, body, err)
	}
}
