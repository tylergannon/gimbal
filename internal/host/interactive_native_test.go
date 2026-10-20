package host

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
)

// This qualification uses the real native executor and requires an explicit opt-in.
func TestNativeInteractiveWaitingSiblingAndOtherProject(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_NATIVE_INTERACTIVE") != "1" {
		t.Skip("set GIMBAL_TEST_NATIVE_INTERACTIVE=1 to run Codex luna")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	owner := New(ctx, t.TempDir())
	defer owner.Close()
	a, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := owner.AdmitProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- a.Run(ctx, "native-interactive-siblings", map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleAgenticDialogues: model}, func(ctx context.Context) error {
			group := gimbal.Group(ctx, "parallel")
			group.Go("discussion", func(ctx context.Context) error {
				session := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, a.Path())
				transcript, err := gimbal.Interactive(ctx, "discussion", session, "When the human asks to finish, return the end variant. Otherwise reply using the chat variant.")
				if err != nil {
					return err
				}
				if transcript.EndedBy != "agent" {
					return fmt.Errorf("ended by %q", transcript.EndedBy)
				}
				return nil
			})
			group.Go("sibling", func(ctx context.Context) error {
				_, _, _, err := gimbal.RunCommand(ctx, "sibling-work", a.Path(), "sh", "-c", "printf sibling > sibling.txt")
				return err
			})
			if err := group.Wait(); err != nil {
				return err
			}
			_, _, _, err := gimbal.RunCommand(ctx, "joined", a.Path(), "sh", "-c", "printf joined > joined.txt")
			return err
		})
	}()
	wait := func(description string, ready func() bool) {
		t.Helper()
		for !ready() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", description, ctx.Err())
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	var id, address string
	wait("waiting discussion and completed sibling", func() bool {
		ids, _ := a.Registry().IDs()
		for _, candidate := range ids {
			snapshot, e := a.Registry().Snapshot(candidate)
			if e != nil {
				continue
			}
			for _, row := range snapshot.Interactions {
				if row.Status == "pending" {
					id, address = candidate, row.Address
				}
			}
		}
		_, err := os.Stat(filepath.Join(a.Path(), "sibling.txt"))
		return id != "" && err == nil
	})
	if _, err := os.Stat(filepath.Join(a.Path(), "joined.txt")); !os.IsNotExist(err) {
		t.Fatalf("join ran while child waited: %v", err)
	}
	if err := b.Run(ctx, "other-project", nil, func(ctx context.Context) error {
		_, _, _, err := gimbal.RunCommand(ctx, "other-work", b.Path(), "sh", "-c", "printf other > other.txt")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	controller, err := a.Runs().InProgress(id)
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"id": "native-finish-request", "message": "Finish now: return the end variant."})
	if err := controller.SubmitInteractive(address, string(input)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	content, err := os.ReadFile(filepath.Join(a.Path(), "joined.txt"))
	if err != nil || string(content) != "joined" {
		t.Fatalf("joined=%q err=%v", content, err)
	}
	snapshot, err := a.Registry().Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	for key, session := range snapshot.Sessions {
		t.Logf("model alias=luna run=%s session=%s native=%+v", id, key, session)
	}
	t.Logf("waiting child resumed, sibling completed before input, another project completed while waiting, and join completed: run=%s address=%s", id, address)
}
