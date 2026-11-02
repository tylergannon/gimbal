package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
)

// TestNativeGroupSessionsLive exercises concurrent Group sessions through native workers.
func TestNativeGroupSessionsLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	owner := New(ctx, t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("luna resolved to %s", model.Model)
	err = project.Run(ctx, "native-concurrent-sessions", map[gimbal.WorkflowRole]gimbal.ModelBinding{"worker": model}, func(ctx context.Context) error {
		group := gimbal.Group(ctx, "parallel")
		for i := range 2 {
			group.Go("worker", func(ctx context.Context) error {
				own := fmt.Sprintf("ready-%d", i)
				other := fmt.Sprintf("ready-%d", 1-i)
				crossed := fmt.Sprintf("crossed-%d", i)
				gimbal.Set(ctx, "shell command", fmt.Sprintf("touch %q; for i in $(seq 1 30); do if test -f %q; then touch %q; exit 0; fi; sleep 1; done; exit 1", own, other, crossed))
				_, err := gimbal.NewSession(ctx, "worker", project.Path()).Generate[gimbal.Text](ctx, "Run the supplied shell command exactly once, then report the result.")
				return err
			})
		}
		return group.Wait()
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if _, err := os.Stat(filepath.Join(project.Path(), fmt.Sprintf("crossed-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
}
