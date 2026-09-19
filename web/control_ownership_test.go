package web

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestControlRunsBelongToThisRuntime(t *testing.T) {
	project, err := os.MkdirTemp("/tmp", "gimble-owners-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(project) })
	ctx, cancel := context.WithCancel(t.Context())
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	first, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	for _, runtime := range []*Runtime{first, second} {
		workers.Go(func() {
			_ = runtime.Run(ctx, "owned", nil, func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			})
		})
	}
	<-started
	<-started
	firstRuns, err := first.controlRuns()
	if err != nil {
		t.Fatal(err)
	}
	secondRuns, err := second.controlRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(firstRuns) != 1 || len(secondRuns) != 1 {
		t.Fatalf("each runtime must advertise only its own active run: first=%d second=%d", len(firstRuns), len(secondRuns))
	}
	if firstRuns[0].ID == secondRuns[0].ID {
		t.Fatal("different runtimes advertised the same run")
	}
}
