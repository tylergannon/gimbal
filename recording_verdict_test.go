package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		completeOnly  bool
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
			name:          "first store failure is Complete itself",
			sabotage:      func(_ *testing.T, _, _ string, fails *atomic.Bool) error { fails.Store(true); return nil },
			completeOnly:  true,
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
			store := faultStore{Store: directory.Store{Root: filepath.Join(project, "runs")}, beforeAppend: func(facts []runstore.Fact) error {
				if storeFails.Load() {
					if tc.completeOnly {
						for _, fact := range facts {
							if fact.Kind == "lifecycle" && strings.Contains(string(fact.Payload), `"kind":"complete"`) {
								return errors.New("disk gone")
							}
						}
						return nil
					}
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

// Fail real blob creation only after the native turn has started. Its valid
// result still reaches the workflow; provider failure keeps its own meaning.
func TestMidTurnRecordingFailureKeepsNativeVerdict(t *testing.T) {
	for _, providerFails := range []bool{false, true} {
		for _, eventFails := range []bool{false, true} {
			t.Run(fmt.Sprintf("provider_failure=%v/event_recording=%v", providerFails, eventFails), func(t *testing.T) {
				project := t.TempDir()
				id := string(runstore.NewRunID())
				registry := observation.NewRegistry(project)
				ctx := observation.WithRegistry(Project(t.Context(), project), registry)
				answer := strings.Repeat("x", 600<<10)
				providerErr := errors.New("provider refused turn")
				adapter := &fake{answer: func(_ context.Context, _, _ string, _ json.RawMessage, emit func(AgentEvent) error) (string, error) {
					blobs := filepath.Join(project, "runs", id, "state", "blobs")
					if err := os.RemoveAll(blobs); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(blobs, []byte("unavailable"), 0600); err != nil {
						t.Fatal(err)
					}
					if eventFails {
						raw, _ := json.Marshal(map[string]string{"sessionID": "native", "text": answer})
						if err := emit(nativeEvent("session.text.started", json.RawMessage(raw), map[string]any{"provider": "fake", "sessionID": "native"})); err != nil {
							return "", err
						}
					}
					if providerFails {
						return "", providerErr
					}
					return answer, nil
				}}
				var got Text
				err := Run(ctx, id, "mid-turn-recording", bind(adapter, "test", "agent"), func(ctx context.Context) error {
					var err error
					got, err = NewSession(ctx, "agent", project).Generate[Text](ctx, "answer")
					return err
				})
				if providerFails {
					if !errors.Is(err, providerErr) {
						t.Fatalf("Run=%v, want provider failure", err)
					}
				} else if err != nil || string(got) != answer {
					t.Fatalf("Run=%v, output bytes=%d, want %d", err, len(got), len(answer))
				}
				snapshot, err := registry.Snapshot(id)
				if err != nil {
					t.Fatal(err)
				}
				status := observation.StatusCompleted
				if providerFails {
					status = observation.StatusFailed
				}
				if snapshot.Run.Status != status {
					t.Errorf("status=%q, want %q", snapshot.Run.Status, status)
				}
				// A small provider error only reaches the broken blob path when the
				// event itself needed a blob. All successful results need one.
				if (!providerFails || eventFails) && !strings.Contains(snapshot.Run.RecordingError, "not a directory") {
					t.Errorf("recording_error=%q", snapshot.Run.RecordingError)
				}
			})
		}
	}
}
