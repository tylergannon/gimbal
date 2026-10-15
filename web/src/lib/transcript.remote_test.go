package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/transcript"
	"github.com/tylergannon/polytype"
)

func TestSavedRecordingWatchYieldsFinalReceiptAndReleasesConnection(t *testing.T) {
	owner := host.New(t.Context(), t.TempDir())
	defer owner.Close()
	project, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := project.Recordings()
	for _, state := range []string{"stopped", "interrupted"} {
		if _, err := store.Open(transcript.OpenRequest{SessionID: state, Source: "synthetic"}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Accept(transcript.Update{SessionID: state, Sequence: 1, Finalized: []transcript.Chunk{{StartMs: 0, EndMs: 100, Text: "Saved final words."}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.State(state, state, nil, polytype.Nullable[int64]{}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(host.WithOwner(t.Context(), owner), time.Second)
		yields := 0
		err := watchRecording(ctx, RecordingRequest{Project: project.ID(), ID: state}, func(record transcript.Recording) error {
			yields++
			if record.Status != state || record.Sequence != 1 || len(record.Finalized) != 1 {
				t.Fatalf("wrong terminal receipt: %+v", record)
			}
			return nil
		})
		if err != nil || ctx.Err() != nil || yields != 1 {
			t.Fatalf("terminal stream stayed open: state=%s yields=%d err=%v ctx=%v", state, yields, err, ctx.Err())
		}
		cancel()
	}
	// Ending the live connection does not freeze mutable requirements metadata.
	if _, err := store.MarkRequirements("stopped"); err != nil {
		t.Fatal(err)
	}
	record, err := recordingSnapshot(host.WithOwner(t.Context(), owner), RecordingRequest{Project: project.ID(), ID: "stopped"})
	if err != nil || !record.Requirements {
		t.Fatalf("snapshot missed external requirements change: %+v %v", record, err)
	}
}
