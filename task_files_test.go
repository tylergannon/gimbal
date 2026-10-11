package gimbal

import (
	"context"
	"strings"
	"testing"
)

func TestRemoteTaskFilesTransfersNestedOwnedMarkersOnly(t *testing.T) {
	err := Run(Project(context.Background(), t.TempDir()), "task-files", nil, func(ctx context.Context) error {
		s, _ := current(ctx)
		value, err := s.run.writeContentArtifact("value", "actual full context")
		if err != nil {
			return err
		}
		index, err := s.run.writeContentArtifact("index", "Complete value: "+artifactAbsolute(s.run, value))
		if err != nil {
			return err
		}
		files, err := RemoteTaskFiles(ctx, "Complete value: "+artifactAbsolute(s.run, index)+"\nComplete value: /not/a/runtime/artifact")
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
