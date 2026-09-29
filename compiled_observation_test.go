package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func TestCompiledObservationFailureDoesNotStopExecution(t *testing.T) {
	for _, operation := range []string{"record plan", "write context"} {
		t.Run(operation, func(t *testing.T) {
			store := compiledscope.Store{Root: t.TempDir()}
			turns := 0
			adapter := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
				turns++
				if operation == "write context" && !strings.Contains(prompt, "accepted input") {
					t.Error("subsequent turn lost the committed context")
				}
				return "completed", nil
			}}
			var bodyErr, recordingErr error
			err := runTest(t, bind(adapter, "fake", "worker"), func(ctx context.Context) error {
				if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
					return err
				}
				bodyErr = func() error {
					loop, finish, err := compiledscope.OpenLoop(ctx, "loop")
					if err != nil {
						return err
					}
					defer func() { _ = finish(nil) }()
					s, _ := current(loop)
					if err := s.run.writer.file.Close(); err != nil {
						return err
					}
					var input compiledscope.Snapshot
					switch operation {
					case "record plan":
						err = compiledscope.RecordPlan(loop, "goal", []byte(`{"tasks":[]}`))
					case "write context":
						input, err = compiledscope.WriteContext(loop, "", compiledscope.Entry{Key: "input", Value: json.RawMessage(`"accepted input"`)})
					}
					if err != nil {
						return err
					}
					recordingErr = s.run.recordingError()
					session := NewSession(loop, "worker", ".")
					raw, err := compiledscope.Generate[Text](loop, session, input, "Continue the work.")
					if err == nil && string(raw) != `"completed"` {
						t.Errorf("authoritative result = %s", raw)
					}
					return err
				}()
				return bodyErr
			})
			if bodyErr != nil || turns != 1 {
				t.Fatalf("execution stopped: body error=%v turns=%d", bodyErr, turns)
			}
			if !errors.Is(recordingErr, os.ErrClosed) || !errors.Is(err, recordingErr) {
				t.Fatalf("recording failure lost: recording=%v run=%v", recordingErr, err)
			}
		})
	}
}
