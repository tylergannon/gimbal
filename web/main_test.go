package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Instance tests use fake harnesses, not the external Jev service.
	if os.Getenv("GIMBAL_LIVE") != "1" {
		_ = os.Setenv("TYPESAFE_API_KEY", "test-key")
	}
	os.Exit(m.Run())
}

func TestInstanceRequiresTypeSafeKeyBeforeStarting(t *testing.T) {
	for _, key := range []string{"", " \t\n"} {
		t.Run("missing-or-blank", func(t *testing.T) {
			t.Setenv("TYPESAFE_API_KEY", key)
			dir := filepath.Join(t.TempDir(), "instance")
			instance, err := NewInstance(t.Context(), dir, nil, WithNoWeb())
			if err == nil || !strings.Contains(err.Error(), "TYPESAFE_API_KEY is required") || instance != nil {
				t.Fatalf("instance=%v error=%v", instance, err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("startup created state without a key: %v", err)
			}
		})
	}
}
