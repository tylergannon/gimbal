package researcheval

import (
	"context"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/runstore"
)

func workflowContext(t *testing.T) context.Context {
	t.Helper()
	ready := make(chan context.Context, 1)
	finish := make(chan struct{})
	done := make(chan error, 1)
	project := t.TempDir()
	go func() {
		done <- gimbal.Run(gimbal.Project(t.Context(), project), string(runstore.NewRunID()), "workflow-helper-test", nil, func(ctx context.Context) error { ready <- ctx; <-finish; return nil })
	}()
	ctx := <-ready
	t.Cleanup(func() {
		close(finish)
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return ctx
}
