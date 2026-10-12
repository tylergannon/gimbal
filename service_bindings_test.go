package gimbal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRejectsServiceDependenciesWithoutMessagingBeforeSideEffects(t *testing.T) {
	project := t.TempDir()
	marker := filepath.Join(project, "must-not-exist")
	err := Run(Project(t.Context(), project), "unsupported-service", nil, func(ctx context.Context) error {
		return Check(WithServices(ctx, "http"), "probe", project, "touch", marker)
	})
	if err == nil {
		t.Fatal("Check silently ignored its named service dependency")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("Check launched unsupported command", err)
	}
}
