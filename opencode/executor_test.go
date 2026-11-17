package opencode

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal"
)

func TestExecutorRetainsNativeConversationAcrossAdapters(t *testing.T) {
	fake := newFakeOpenCode(t)
	config := adapterConfig{
		stateDir: t.TempDir(),
		connect: func(_ context.Context, workdir, _ string) (*Client, error) {
			return newClient(fake.server.URL, "user", "secret", workdir, fake.server.Client()), nil
		},
	}
	first := newAdapter(config)
	id, err := first.CreateSession(t.Context(), "anthropic/claude-test", "low", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.RunTurn(t.Context(), id, "remember pear", nil, func(gimbal.AgentEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	checkpoint := filepath.Join(t.TempDir(), "native.json")
	if err := first.SaveSession(t.Context(), id, checkpoint); err != nil {
		t.Fatal(err)
	}
	if err := first.DetachSession(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	second := newAdapter(config)
	resumed, err := second.LoadSession(t.Context(), checkpoint, false)
	if err != nil || resumed != id {
		t.Fatalf("load = %q (%v), want %q", resumed, err, id)
	}
	t.Cleanup(func() { _ = second.Close(context.Background(), resumed) })
	result, err := second.RunTurn(t.Context(), resumed, "CONTINUE", nil, func(gimbal.AgentEvent) error { return nil })
	if err != nil || string(result.Output) != `"pear"` {
		t.Fatalf("continued = %s (%v)", result.Output, err)
	}
}
