package web

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestControlRunsBelongToThisProject(t *testing.T) {
	base := t.TempDir()
	projectA, projectB := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, project := range []string{projectA, projectB} {
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	instance, err := NewInstance(ctx, filepath.Join(base, "instance"), WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	first, err := instance.AdmitProject(projectA)
	if err != nil {
		t.Fatal(err)
	}
	second, err := instance.AdmitProject(projectB)
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
		t.Fatalf("each project must advertise only its own active run: first=%d second=%d", len(firstRuns), len(secondRuns))
	}
	if firstRuns[0].ID == secondRuns[0].ID {
		t.Fatal("different projects advertised the same run")
	}
}
