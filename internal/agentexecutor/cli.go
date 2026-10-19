package agentexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/internal/modelalias"
)

func CLI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gimbal agent-executor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var root, id, sessionID, harness, model, effort, workdir, prompt, promptFile, schema, schemaFile, lookup, cancel, steer string
	var timeout time.Duration
	var closeSession string
	var vacuum bool
	flags.StringVar(&root, "state-dir", "", "executor state directory")
	flags.StringVar(&id, "turn", "", "stable turn_ ULID")
	flags.StringVar(&sessionID, "session", "", "stable session_ ULID")
	flags.StringVar(&harness, "harness", "", "pi, claude, codex, or agy")
	flags.StringVar(&model, "model", "", "native model name")
	flags.StringVar(&effort, "effort", "", "native reasoning effort")
	flags.StringVar(&workdir, "workdir", ".", "native workspace")
	flags.StringVar(&prompt, "prompt", "", "turn prompt")
	flags.StringVar(&promptFile, "prompt-file", "", "snapshot prompt from file")
	flags.StringVar(&schema, "schema", "", "output JSON Schema")
	flags.StringVar(&schemaFile, "schema-file", "", "snapshot schema from file")
	flags.DurationVar(&timeout, "timeout", 20*time.Minute, "accepted execution deadline; 0 disables")
	flags.StringVar(&lookup, "lookup", "", "look up turn identity")
	flags.StringVar(&cancel, "cancel", "", "cancel turn identity")
	flags.StringVar(&steer, "steer", "", "steer message for --lookup turn")
	flags.StringVar(&closeSession, "close", "", "close a session and release native resources")
	flags.BoolVar(&vacuum, "vacuum", false, "remove terminal operations older than seven days; preserve all session state")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	root, err := executor.Root(root)
	if err != nil {
		return err
	}
	if vacuum {
		return executor.Vacuum(root, time.Now())
	}
	if closeSession != "" {
		adapter := Adapter{Root: root}
		if err = adapter.Close(context.Background(), closeSession); err != nil {
			return err
		}
		sdir := sessionDir(root, executor.SessionID(closeSession))
		return json.NewEncoder(stdout).Encode(executor.Descriptor{State: "completed", Socket: nil, Events: filepath.Join(sdir, "closed-events.jsonl"), Result: filepath.Join(sdir, "closed.json")})
	}

	if lookup != "" || cancel != "" {
		if cancel != "" {
			lookup = cancel
		}
		if err = executor.Validate(lookup, "turn_"); err != nil {
			return err
		}
		dir := filepath.Join(root, lookup)
		if !executor.Exists(filepath.Join(dir, "request.json")) {
			return errors.New("turn not found")
		}
		executor.AssertOwner(dir)
		if cancel != "" || steer != "" {
			op := "steer"
			if cancel != "" {
				op = "cancel"
			}
			if !executor.Exists(filepath.Join(dir, "result.json")) {
				if _, err = executor.Call(context.Background(), dir, executor.Control{Op: op, Message: steer}); err != nil {
					return err
				}
			}
		}
		return json.NewEncoder(stdout).Encode(executor.Describe(dir, true))
	}
	if promptFile != "" {
		raw, err := os.ReadFile(promptFile)
		if err != nil {
			return err
		}
		prompt = string(raw)
	}
	if schemaFile != "" {
		raw, err := os.ReadFile(schemaFile)
		if err != nil {
			return err
		}
		schema = string(raw)
	}
	if !json.Valid([]byte(schema)) && schema != "" {
		return errors.New("invalid schema JSON")
	}
	if model != "" {
		selection := modelalias.Selection{Name: model}
		if effort != "" {
			selection.Effort = effort
			selection.EffortPresent = true
		}
		resolved, e := modelalias.Resolve(selection)
		if e != nil {
			return e
		}
		if harness != "" && harness != resolved.Harness {
			return errors.New("model does not belong to requested harness")
		}
		harness, model, effort = resolved.Harness, resolved.Model, resolved.Effort
	}
	if harness != "" {
		if err = Create(root, Session{ID: executor.SessionID(sessionID), Harness: harness, Model: model, Effort: effort, Workdir: workdir}); err != nil {
			return err
		}
	}
	req := Request{ID: executor.TurnID(id), Session: executor.SessionID(sessionID), Prompt: prompt, Schema: json.RawMessage(schema), Timeout: timeout}
	d, err := Submit(root, req)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(d)
}
