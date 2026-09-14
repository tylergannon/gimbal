// This file is a frozen record of the probes run for the PR 140 Loop
// review (see ../REVIEW.md and ../../claude-loop-api/REVIEW.md): it is kept
// as evidence of what ran against the HarnessAdapter contract at that time,
// not maintained against later interface changes (issue 112 found it no
// longer compiles after Close and TurnResult landed). The build tag keeps
// it out of `go build ./...` and `go vet ./...` without altering the record.
//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tylergannon/gimble"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fake struct {
	calls  int
	answer func(int, string) (json.RawMessage, error)
}

func (f *fake) CreateSession(context.Context, string, string) (string, error) { return "fake", nil }
func (f *fake) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	f.calls++
	out, err := f.answer(f.calls, prompt)
	return gimble.TurnResult{Output: out}, err
}
func (f *fake) Steer(context.Context, string, string) (bool, error) {
	return false, errors.New("steer delivery failed")
}
func (f *fake) Fork(context.Context, string) (string, error) { return "fork", nil }
func (f *fake) Close(context.Context, string) error          { return nil }
func backlog(p string) string {
	_, s, _ := strings.Cut(p, "Its revisable backlog is ")
	s, _, _ = strings.Cut(s, ".\n")
	return s
}
func choose(p string) json.RawMessage {
	t := gimble.Task{Name: "implement", Description: "implement the behavior", DefinitionOfDone: "acceptance passes"}
	b, _ := json.Marshal(map[string]any{"goal": "goal", "tasks": []gimble.Task{t}})
	if err := os.WriteFile(backlog(p), append(append([]byte("---\n"), b...), []byte("\n---\n")...), 0644); err != nil {
		panic(err)
	}
	out, _ := json.Marshal(map[string]any{"next": t})
	return out
}

type result struct{}

func (result) Schema() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (result) ValidateJSON([]byte) error { return errors.New("intentional schema rejection") }
func run(base, name string, body func(context.Context) error) {
	dir := filepath.Join(base, name)
	err := gimble.Run(gimble.Project(context.Background(), dir), name, body)
	fmt.Printf("%s Run error: %v\n", name, err)
}
func main() {
	base, _ := filepath.Abs("ephemeral/review/loop-api/probe-artifacts")
	os.MkdirAll(base, 0755)
	f := &fake{}
	f.answer = func(n int, p string) (json.RawMessage, error) {
		if n == 1 {
			return choose(p), nil
		}
		fmt.Printf("nested feedback: direct=%t nested=%t\n", strings.Contains(p, "DIRECT_SENTINEL"), strings.Contains(p, "NESTED_FAILURE_SENTINEL"))
		return json.RawMessage(`{"next":null}`), nil
	}
	run(base, "nested", func(ctx context.Context) error {
		l := gimble.Loop(ctx, "loop", "goal", gimble.NewSession(ctx, "planner", f, "fake", "."))
		for taskCtx := range l.Tasks {
			gimble.Set(taskCtx, "result", "DIRECT_SENTINEL")
			if err := gimble.Scope(taskCtx, "validator", func(c context.Context) error { gimble.Set(c, "failed evidence", "NESTED_FAILURE_SENTINEL"); return nil }); err != nil {
				return err
			}
		}
		return l.Err()
	})
	f = &fake{}
	f.answer = func(n int, p string) (json.RawMessage, error) {
		if n == 9 {
			return nil, errors.New("probe stops after 8 malformed planning answers")
		}
		os.WriteFile(backlog(p), []byte("invalid"), 0644)
		return json.RawMessage(`{"next":null}`), nil
	}
	run(base, "repair", func(ctx context.Context) error {
		l := gimble.Loop(ctx, "loop", "goal", gimble.NewSession(ctx, "planner", f, "fake", "."))
		tasks := 0
		for range l.Tasks {
			tasks++
			if tasks == 1 {
				break
			}
		}
		fmt.Printf("repair: yielded tasks=%d planner calls=%d despite task limit=1\n", tasks, f.calls)
		return l.Err()
	})
	f = &fake{answer: func(int, string) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }}
	run(base, "schema", func(ctx context.Context) error {
		_, err := gimble.NewSession(ctx, "worker", f, "fake", ".").Generate[result](ctx, "return result")
		return err
	})
	f = &fake{answer: func(_ int, p string) (json.RawMessage, error) { return choose(p), nil }}
	run(base, "body-error", func(ctx context.Context) error {
		l := gimble.Loop(ctx, "loop", "goal", gimble.NewSession(ctx, "planner", f, "fake", "."))
		for range l.Tasks {
			return errors.New("worker failed in range body")
		}
		return l.Err()
	})
	for _, name := range []string{"schema", "body-error"} {
		files, _ := filepath.Glob(filepath.Join(base, name, "runs", "*", "run.jsonl"))
		raw, _ := os.ReadFile(files[len(files)-1])
		for _, line := range strings.Split(string(raw), "\n") {
			var r struct {
				Scope string         `json:"scope"`
				Event map[string]any `json:"event"`
			}
			if json.Unmarshal([]byte(line), &r) != nil {
				continue
			}
			k := r.Event["kind"]
			if k == "turn_ended" || k == "scope_ended" || k == "run_ended" {
				fmt.Printf("%s record scope=%q %s\n", name, r.Scope, line)
			}
		}
	}
	// A cancelled run's final event suffix, consumed by the real browser reducer below.
	c, cancel := context.WithCancel(context.Background())
	p := filepath.Join(base, "cancelled")
	err := gimble.Run(gimble.Project(c, p), "cancelled", func(ctx context.Context) error { cancel(); return ctx.Err() })
	fmt.Println("cancelled Run error:", err)
	files, _ := filepath.Glob(filepath.Join(p, "runs", "*", "run.jsonl"))
	raw, _ := os.ReadFile(files[len(files)-1])
	os.WriteFile(filepath.Join(base, "cancelled-events.jsonl"), raw, 0644)
	files, _ = filepath.Glob(filepath.Join(p, "runs", "*", "observation.json"))
	raw, _ = os.ReadFile(files[len(files)-1])
	os.WriteFile(filepath.Join(base, "cancelled-snapshot.json"), raw, 0644)
	fmt.Println("Finished", time.Now().UTC())
}
