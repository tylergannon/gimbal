package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
)

type planValue struct {
	Tasks []gimbal.Task `json:"tasks"`
	Next  *int          `json:"next"`
}
type planResult struct {
	Value   planValue
	Failure *operationFailure
}

func (r planResult) Err() error {
	if r.Failure == nil {
		return nil
	}
	return r.Failure
}
func (a *Activities) EnterLoopScope(_ context.Context, in ScopeInput) error {
	return a.enterScope(in, compiledscope.OpenLoop)
}
func (a *Activities) EnterTaskScope(_ context.Context, in ScopeInput, task gimbal.Task) error {
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return a.enterScope(in, func(ctx context.Context, name string) (context.Context, func(error) error, error) {
		return compiledscope.OpenTask(ctx, name, raw)
	})
}
func (a *Activities) plan(ctx context.Context, in operationInput, name, goal string, tasks []gimbal.Task, previous string) (planResult, error) {
	scoped, release, err := a.input(ctx, in.Scope, in.Context)
	if err != nil {
		return planResult{}, err
	}
	defer release()
	session, err := a.session(in)
	if err != nil {
		return planResult{}, err
	}
	raw, err := json.Marshal(tasks)
	if err != nil {
		return planResult{}, err
	}
	answer, err := compiledscope.Plan(scoped, session, name, goal, raw, previous)
	var out planValue
	if err == nil {
		err = json.Unmarshal(answer, &out)
	}
	return planResult{out, failure(err)}, nil
}
func (a *Activities) RecordPlan(ctx context.Context, id, goal string, p planValue) error {
	scoped, release, err := a.operation(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return compiledscope.RecordPlan(scoped, goal, raw)
}

type checkResult struct {
	commandResult
	Context compiledscope.Snapshot
}

func (a *Activities) check(ctx context.Context, in operationInput, key, command string, args ...string) (checkResult, error) {
	return a.checkAt(ctx, in, key, a.workdir, command, args...)
}
func (a *Activities) checkAt(ctx context.Context, in operationInput, key, dir, command string, args ...string) (checkResult, error) {
	scoped, release, err := a.input(ctx, in.Scope, in.Context)
	if err != nil {
		return checkResult{}, err
	}
	defer release()
	code, out, stderr, commandErr := gimbal.RunCommand(scoped, key, dir, command, args...)
	record := checkRecord{command, append([]string(nil), args...), dir, code, out, stderr, errorText(commandErr)}
	// Keep the producing lease until both command and its implicit Set finish.
	// Context publication is independent of a cancelled process context.
	ref, recordErr := compiledscope.WriteContext(scoped, a.store, in.Context, contextEntry(key, record))
	return checkResult{commandResult{code, out, stderr, failure(errors.Join(commandErr, recordErr))}, ref}, nil
}
func (a *Activities) TaskFeedback(ctx context.Context, in operationInput, keys []string) (string, error) {
	entries, err := a.store.Load(in.Context)
	if err != nil {
		return "", err
	}
	var local []compiledscope.Entry
	for _, e := range entries {
		if slices.Contains(keys, e.Key) {
			local = append(local, e)
		}
	}
	ref, err := a.store.Extend("", local...)
	if err != nil {
		return "", err
	}
	scoped, release, err := a.input(ctx, in.Scope, ref)
	if err != nil {
		return "", err
	}
	defer release()
	return compiledscope.TaskFeedback(scoped)
}

// Check's complete public record, including failed execution, is written even
// when the workflow will then propagate that execution error.
type checkRecord struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Workdir  string   `json:"workdir"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	Error    string   `json:"error,omitempty"`
}
