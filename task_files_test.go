package gimbal

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"

	"github.com/tylergannon/gimbal/runstore"
)

func TestRemoteTaskFilesTransfersNestedOwnedMarkersOnly(t *testing.T) {
	err := Run(Project(context.Background(), t.TempDir()), string(runstore.NewRunID()), "task-files", nil, func(ctx context.Context) error {
		s, _ := current(ctx)
		value, err := s.run.writeContentArtifact("value", "actual full context")
		if err != nil {
			return err
		}
		index, err := s.run.writeContentArtifact("index", "Complete value: "+artifactAbsolute(s.run, value))
		if err != nil {
			return err
		}
		files, err := remoteTaskFiles(ctx, "Complete value: "+artifactAbsolute(s.run, index)+"\nComplete value: /not/a/runtime/artifact")
		if err != nil {
			return err
		}
		if len(files) != 2 {
			t.Fatalf("got %d owned files", len(files))
		}
		if string(files[1].Bytes) != "actual full context" || !strings.Contains(string(files[0].Bytes), files[1].Reference) {
			t.Fatal("nested context bytes lost")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSessionCollectsExplicitAndOwnedTaskFilesBeforeAdapter(t *testing.T) {
	calls := 0
	adapter := &fake{answer: func(ctx context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		calls++
		files := effects.TaskFiles(ctx)
		if len(files) != 2 || files[0].Name != "explicit" || string(files[0].Bytes) != "author bytes" || string(files[1].Bytes) != "owned bytes" {
			t.Fatalf("adapter files=%+v", files)
		}
		if !strings.Contains(prompt, "Complete value: "+files[1].Reference) {
			t.Fatal("owned reference missing")
		}
		return `"accepted"`, nil
	}}
	err := runTest(t, bind(adapter, "m", "coder"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		artifact, err := scope.run.writeContentArtifact("value", "owned bytes")
		if err != nil {
			return err
		}
		raw := []byte("author bytes")
		ctx = WithTaskFiles(ctx, backend.TaskFile{Name: "explicit", Bytes: raw})
		raw[0] = 'X'
		session := NewSession(ctx, "coder", "/w")
		ctx = effects.WithEndpoint(ctx, "remote")
		_, err = session.turn(ctx, "Complete value: "+artifactAbsolute(scope.run, artifact), nil, nil, "Text", nil, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("adapter calls=%d", calls)
	}
}

func TestLocalTurnDoesNotReadRemoteTransferArtifacts(t *testing.T) {
	adapter := &fake{answer: func(ctx context.Context, _ string, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		if len(effects.TaskFiles(ctx)) != 0 {
			t.Fatal("local adapter received transfer bytes")
		}
		if !strings.Contains(prompt, "Complete value:") {
			t.Fatal("native artifact reference lost")
		}
		return `"accepted"`, nil
	}}
	err := runTest(t, bind(adapter, "m", "coder"), func(ctx context.Context) error {
		scope, _ := current(ctx)
		artifact, err := scope.run.writeContentArtifact("value", "owned bytes")
		if err != nil {
			return err
		}
		path := artifactAbsolute(scope.run, artifact)
		if err := os.Remove(path); err != nil {
			return err
		}
		_, err = NewSession(ctx, "coder", "/w").turn(ctx, "Complete value: "+path, nil, nil, "Text", nil, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
