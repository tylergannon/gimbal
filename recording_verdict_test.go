package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

// What the harness returned decides the verdict. Recording that cannot keep
// up afterwards is shown beside it, in the returned-run snapshot and the run
// log, and never replaces it.
func TestRecordingFailureAfterHarnessResultKeepsExecutionVerdict(t *testing.T) {
	tests := []struct {
		name string
		// sabotage runs inside the workflow after the harness has answered.
		sabotage      func(t *testing.T, project, id string, runStoreFails *atomic.Bool) error
		bodyErr       error
		wantStatus    string
		wantError     string
		wantRecording string
	}{
		{
			name: "observation journal replaced by a directory, harness succeeded",
			sabotage: func(t *testing.T, project, id string, _ *atomic.Bool) error {
				journal := filepath.Join(project, "runs", id, "observation-deltas.jsonl")
				if err := os.Remove(journal); err != nil {
					return err
				}
				return os.Mkdir(journal, 0o755)
			},
			wantStatus:    observation.StatusCompleted,
			wantRecording: "is a directory",
		},
		{
			name: "run store refuses appends, harness succeeded",
			sabotage: func(_ *testing.T, _, _ string, fails *atomic.Bool) error {
				fails.Store(true)
				return nil
			},
			wantStatus:    observation.StatusCompleted,
			wantRecording: "disk gone",
		},
		{
			name: "run store refuses appends, workflow failed",
			sabotage: func(_ *testing.T, _, _ string, fails *atomic.Bool) error {
				fails.Store(true)
				return nil
			},
			bodyErr:       errors.New("workflow rejected the answer"),
			wantStatus:    observation.StatusFailed,
			wantError:     "workflow rejected the answer",
			wantRecording: "disk gone",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project := t.TempDir()
			id := string(runstore.NewRunID())
			var storeFails atomic.Bool
			store := faultStore{Store: directory.Store{Root: filepath.Join(project, "runs")}, beforeAppend: func([]runstore.Fact) error {
				if storeFails.Load() {
					return errors.New("disk gone")
				}
				return nil
			}}
			registry := observation.NewRegistry(project)
			ctx := observation.WithRegistry(WithRunStore(Project(t.Context(), project), store), registry)
			adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
				return "ok", nil
			}}
			err := Run(ctx, id, "recording-verdict", bind(adapter, "test", "agent"), func(ctx context.Context) error {
				got, err := NewSession(ctx, "agent", project).Generate[Text](ctx, "say ok")
				if err != nil {
					return err
				}
				if got != "ok" {
					t.Errorf("harness answered %q, want ok", got)
				}
				if err := tc.sabotage(t, project, id, &storeFails); err != nil {
					t.Fatal(err)
				}
				return tc.bodyErr
			})
			if tc.wantError == "" && err != nil {
				t.Fatalf("Run = %v, want the harness's success", err)
			}
			if tc.wantError != "" && (err == nil || err.Error() != tc.wantError) {
				t.Fatalf("Run = %v, want exactly %q", err, tc.wantError)
			}
			snapshot, err := registry.Snapshot(id)
			if err != nil {
				t.Fatal(err)
			}
			run := snapshot.Run
			if run.Status != tc.wantStatus || run.Error != tc.wantError {
				t.Errorf("snapshot status=%q error=%q, want status=%q error=%q", run.Status, run.Error, tc.wantStatus, tc.wantError)
			}
			if !strings.Contains(run.RecordingError, tc.wantRecording) {
				t.Errorf("snapshot recording error = %q, want it to mention %q", run.RecordingError, tc.wantRecording)
			}
			raw, err := os.ReadFile(filepath.Join(project, "runs", id, "run.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), `"kind":"complete"`) || !strings.Contains(string(raw), tc.wantRecording) {
				t.Errorf("run log does not show the degraded recording:\n%s", raw)
			}
		})
	}
}
