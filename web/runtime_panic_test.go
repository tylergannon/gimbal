package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/observation"
)

func TestDirectRunStillPanicsAfterRecordingFailure(t *testing.T) {
	project := t.TempDir()
	for _, tc := range []struct {
		name, panicText string
		body            func(context.Context) error
	}{
		{"direct-root", "root exploded", func(context.Context) error { panic("root exploded") }},
		{"direct-group", "child exploded", func(ctx context.Context) error {
			group := gimbal.Group(ctx, "children")
			group.Go("child", func(context.Context) error { panic("child exploded") })
			return group.Wait()
		}},
	} {
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = gimbal.Run(gimbal.Project(t.Context(), project), tc.name, nil, tc.body)
		}()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), tc.panicText) {
			t.Fatalf("direct %s panic = %v", tc.name, recovered)
		}
		entries, err := os.ReadDir(filepath.Join(project, "runs"))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), "."+tc.name) {
				continue
			}
			snapshot, err := observation.NewRegistry(project).Snapshot(entry.Name())
			if err != nil || snapshot.Run.Status != observation.StatusFailed || !strings.Contains(snapshot.Run.Error, tc.panicText) {
				t.Fatalf("direct %s history: %+v, %v", tc.name, snapshot.Run, err)
			}
			found = true
		}
		if !found {
			t.Fatalf("missing direct %s run", tc.name)
		}
	}
}
