package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/resulttypes"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

const acceptedResult = `{"state":"ready","detail":{"outcome":{"type":"Success","message":"complete"},"flags":[true,false]},"receipt":"original"}`

func TestPolytypeResponseCorrespondence(t *testing.T) {
	cases := []struct{ name, first, accepted string }{
		{"success", acceptedResult, acceptedResult},
		{"pointer_variant", strings.Replace(acceptedResult, `"type":"Success","message":"complete"`, `"type":"Rejected","reason":"review"`, 1), ""},
		{"duplicate_enum", strings.Replace(acceptedResult, `"state":"ready"`, `"state":"unknown","state":"ready"`, 1), acceptedResult},
		{"duplicate_nested", strings.Replace(acceptedResult, `"flags":[true,false]`, `"flags":7,"flags":[true,false]`, 1), acceptedResult},
		{"bad_enum", strings.Replace(acceptedResult, `"ready"`, `"unknown"`, 1), acceptedResult},
		{"bad_discriminator", strings.Replace(acceptedResult, `"Success"`, `"Unknown"`, 1), acceptedResult},
		{"missing_variant_field", strings.Replace(acceptedResult, `,"message":"complete"`, "", 1), acceptedResult},
		{"bad_nested_scalar", strings.Replace(acceptedResult, `[true,false]`, `[true,7]`, 1), acceptedResult},
		{"null_union", strings.Replace(acceptedResult, `{"type":"Success","message":"complete"}`, `null`, 1), acceptedResult},
		{"exhaustion", `{}`, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.accepted == "" {
				tc.accepted = tc.first
			}
			var results []map[string]json.RawMessage
			for _, target := range []bool{false, true} {
				a := newTestActivities(t)
				attempts := 0
				adapter := &specimenAdapter{turn: func(_ context.Context, _, prompt string) (any, error) {
					attempts++
					if attempts > 1 && !strings.Contains(prompt, "previous answer was invalid") {
						t.Error("re-ask missing")
					}
					raw := tc.first
					if attempts > 1 {
						raw = tc.accepted
					}
					return json.RawMessage(raw), nil
				}}
				a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "deterministic"}}
				var err error
				if target {
					var suite testsuite.WorkflowTestSuite
					env := suite.NewTestWorkflowEnvironment()
					env.RegisterActivity(a)
					env.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: "test"}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
					env.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: "ReleaseEnvironment"})
					env.SetOnActivityCompletedListener(func(info *activity.Info, encoded converter.EncodedValue, activityErr error) {
						if info.ActivityType.Name != "ResultsGenerate1" {
							return
						}
						if activityErr != nil {
							t.Error(activityErr)
							return
						}
						var response responseResult
						if err := encoded.Get(&response); err != nil {
							t.Error(err)
							return
						}
						if tc.name == "exhaustion" {
							if response.Err() == nil || len(response.Value) != 0 {
								t.Error("invalid bytes escaped failed operation")
							}
							return
						}
						if !bytes.Equal(response.Value, []byte(tc.accepted)) {
							t.Errorf("accepted bytes changed: %s", response.Value)
						}
						first, err := compiledscope.Consume[resulttypes.Result](response.Value, response.Err())
						if err != nil {
							t.Error(err)
							return
						}
						second, err := compiledscope.Consume[resulttypes.Result](response.Value, nil)
						if err != nil || !reflect.DeepEqual(first, second) {
							t.Error("repeated consumption differs")
						}
						if first.State != resulttypes.Ready || first.Receipt != "original" || len(first.Detail.Flags) != 2 {
							t.Errorf("typed fields=%+v", first)
						}
						switch outcome := first.Detail.Outcome.(type) {
						case resulttypes.Success:
							if outcome.Message != "complete" {
								t.Error(outcome)
							}
						case *resulttypes.Rejected:
							if outcome.Reason != "review" {
								t.Error(outcome)
							}
						default:
							t.Errorf("union not reconstructed: %T", outcome)
						}
					})
					env.ExecuteWorkflow(ResultsWorkflow, Input{})
					err = env.GetWorkflowError()
				} else {
					err = a.project.Run(t.Context(), "results", a.models, func(ctx context.Context) error { return resulttypes.Results(ctx, gimbal.Env{WorkDir: a.workdir}) })
				}
				if tc.name == "exhaustion" {
					if err == nil || attempts != 3 {
						t.Fatalf("exhaustion: %v attempts=%d", err, attempts)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				wantAttempts := 1
				if tc.first != tc.accepted {
					wantAttempts = 2
				}
				if tc.name == "exhaustion" {
					wantAttempts = 3
				}
				if attempts != wantAttempts {
					t.Fatalf("attempts=%d want %d", attempts, wantAttempts)
				}
				turns := recordedTurns(t, a.project.Dir())
				if len(turns) != attempts {
					t.Fatalf("recorded attempts=%d want %d", len(turns), attempts)
				}
				for i, turn := range turns {
					failed := tc.name == "exhaustion" || i < attempts-1
					if (turn.err != "") != failed {
						t.Errorf("attempt %d error=%q", i, turn.err)
					}
				}
				paths, err := filepath.Glob(filepath.Join(a.project.Dir(), "runs", "*"))
				if err != nil || len(paths) != 1 {
					t.Fatal(paths, err)
				}
				snapshot, err := a.project.Registry().Snapshot(filepath.Base(paths[0]))
				if err != nil {
					t.Fatal(err)
				}
				values := map[string]json.RawMessage{}
				for _, key := range []string{"original", "consumed"} {
					if value, ok := snapshot.Scopes[""].Values[key]; ok {
						values[key] = value.Value
					}
				}
				if tc.name != "exhaustion" {
					var original, consumed resulttypes.Result
					if err := json.Unmarshal(values["original"], &original); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(values["consumed"], &consumed); err != nil {
						t.Fatal(err)
					}
					if original.Receipt != "original" || consumed.Receipt != "consumed:original" {
						t.Fatal("write-time capture changed")
					}
				}
				results = append(results, values)
			}
			if !reflect.DeepEqual(results[0], results[1]) {
				t.Fatalf("source/target writes differ: %s / %s", results[0], results[1])
			}
		})
	}
}

func TestPolytypeSchemaDecoderGap(t *testing.T) {
	type derived struct {
		resulttypes.Result
		Receipt bool `json:"receipt"`
	}
	if err := (derived{}).ValidateJSON([]byte(acceptedResult)); err != nil {
		t.Fatal(err)
	}
	if _, err := compiledscope.Consume[derived]([]byte(acceptedResult), nil); err == nil {
		t.Fatal("promoted schema does not describe the outer result")
	}

	raw := []byte(`{"outcome":{"type":"Collision","TYPE":true}}`)
	if err := (resulttypes.CollisionResult{}).ValidateJSON(raw); err != nil {
		t.Fatalf("pinned discriminator behavior changed: %v", err)
	}
	if _, err := compiledscope.Consume[resulttypes.CollisionResult](raw, nil); err == nil {
		t.Fatal("expected case-folded discriminator decode failure")
	}

	for _, raw := range []string{`{"count":128}`, `{"count":1.0}`, `{"count":1e0}`} {
		if err := (resulttypes.Numeric{}).ValidateJSON([]byte(raw)); err != nil {
			t.Fatalf("pinned schema behavior changed: %v", err)
		}
		if _, err := compiledscope.Consume[resulttypes.Numeric]([]byte(raw), nil); err == nil {
			t.Fatalf("expected decoder constraint for %s", raw)
		}
	}
}

// This runs the generated workflow on a real Temporal server, then replays its
// recorded activity bytes with the workers stopped. No paid provider is involved.
func TestLiveGeneratedResponseReplay(t *testing.T) {
	address := os.Getenv("SPECIMEN_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("set SPECIMEN_TEMPORAL_ADDRESS to an isolated Temporal dev server")
	}
	root := t.TempDir()
	c, err := client.Dial(client.Options{HostPort: address, DataConverter: dataConverter(root)})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("response-replay-%d", time.Now().UnixNano())
	a := newTestActivities(t)
	a.store = compiledscope.Store{Root: filepath.Join(root, environmentID(id), "context")}
	attempts := 0
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Model: "deterministic", Adapter: &specimenAdapter{turn: func(context.Context, string, string) (any, error) {
		attempts++
		if attempts == 1 {
			return json.RawMessage(`{}`), nil
		}
		return json.RawMessage(acceptedResult), nil
	}}}}
	w := worker.New(c, id, worker.Options{})
	w.RegisterWorkflow(ResultsWorkflow)
	w.RegisterActivity(a)
	control := worker.New(c, controlQueue, worker.Options{})
	control.RegisterActivityWithOptions(func(context.Context) (Environment, error) { return Environment{Queue: id}, nil }, activity.RegisterOptions{Name: "ProvisionEnvironment"})
	control.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: "ReleaseEnvironment"})
	if err = control.Start(); err != nil {
		t.Fatal(err)
	}
	if err = w.Start(); err != nil {
		control.Stop()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: id}, ResultsWorkflow, Input{})
	if err == nil {
		err = run.Get(ctx, nil)
		if err != nil {
			_ = c.CancelWorkflow(context.WithoutCancel(ctx), id, run.GetRunID())
		}
	}
	w.Stop()
	control.Stop()
	if err != nil {
		t.Fatal(err)
	}
	history, err := recordedHistory(ctx, c, id)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{DataConverter: dataConverter(root)})
	if err != nil {
		t.Fatal(err)
	}
	replay.RegisterWorkflow(ResultsWorkflow)
	for range 2 {
		if err = replay.ReplayWorkflowHistory(nil, history); err != nil {
			t.Fatal(err)
		}
	}
	if attempts != 2 {
		t.Fatalf("replay dispatched an agent or retry changed: %d", attempts)
	}
	t.Logf("generated workflow completed after one validation re-ask; %d history events replayed twice with workers stopped", len(history.Events))
}
