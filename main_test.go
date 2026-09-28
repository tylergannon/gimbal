package gimbal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Unit tests use fake harnesses; live tests must supply actual credentials.
	if os.Getenv("GIMBAL_LIVE") != "1" {
		_ = os.Setenv("TYPESAFE_API_KEY", "test-key")
	}
	os.Exit(m.Run())
}

func TestRunRequiresTypeSafeKeyBeforeStarting(t *testing.T) {
	for _, key := range []string{"", " \t\n"} {
		t.Run("missing-or-blank", func(t *testing.T) {
			t.Setenv("TYPESAFE_API_KEY", key)
			dir := filepath.Join(t.TempDir(), "project")
			called := false
			err := Run(Project(t.Context(), dir), "no-key", nil, func(context.Context) error {
				called = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "TYPESAFE_API_KEY is required") || called {
				t.Fatalf("error=%v workflow called=%t", err, called)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("startup created state without a key: %v", err)
			}
		})
	}
}
