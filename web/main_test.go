package web

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	// Instance tests use fake harnesses, not the external Jev service.
	if os.Getenv("GIMBAL_LIVE") != "1" {
		_ = os.Setenv("TYPESAFE_API_KEY", "test-key")
	}
	os.Exit(m.Run())
}

func TestInstanceStartsWithoutTypeSafeKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	ctx, cancel := context.WithCancel(t.Context())
	instance, err := NewInstance(ctx, filepath.Join(t.TempDir(), "instance"), nil, WithNoWeb())
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	<-instance.done
}
