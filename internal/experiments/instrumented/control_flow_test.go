package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// These are deliberately two programs, not a shared interpreter. The first
// uses Go's real range-over-function exits; the second is handwritten lowering.
func sourceExits(ctx context.Context, mode string) (visits []string, err error) {
outer:
	for outer, n := range gimbal.Iterate(ctx, "outer", []int{0, 1}) {
		for _, m := range gimbal.Iterate(outer, "inner", []int{0, 1}) {
			visits = append(visits, fmt.Sprintf("%d/%d", n, m))
			if n == 0 && m == 0 {
				switch mode {
				case "continue":
					continue
				case "break":
					break
				}
				// A break in a switch targets the switch, so the loop exits live here.
				if mode == "break" {
					break
				}
				if mode == "continue-outer" {
					continue outer
				}
				if mode == "break-outer" {
					break outer
				}
				if mode == "return" {
					return append(visits, "return-value"), nil
				}
				if mode == "error" {
					return visits, errors.New("body failure")
				}
			}
		}
		visits = append(visits, "after-inner")
	}
	return append(visits, "after-outer"), nil
}
func loweredExits(ctx workflow.Context, mode string) (visits []string, err error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	wait, _ := workflow.NewDisconnectedContext(ctx)
	closeScope := func(id string) error {
		cleanup := cleanupContext(ctx)
		return workflow.ExecuteActivity(cleanup, "ExitScope", id, "").Get(cleanup, nil)
	}
outer:
	for n := range 2 {
		outer := fmt.Sprintf("outer.%d", n+1)
		if err = workflow.ExecuteActivity(ctx, "EnterScope", ScopeInput{outer, "", "outer"}).Get(wait, nil); err != nil {
			return
		}
		for m := range 2 {
			inner := fmt.Sprintf("%s/inner.%d", outer, m+1)
			if err = workflow.ExecuteActivity(ctx, "EnterScope", ScopeInput{inner, outer, "inner"}).Get(wait, nil); err != nil {
				err = errors.Join(err, closeScope(outer))
				return
			}
			visits = append(visits, fmt.Sprintf("%d/%d", n, m))
			if n == 0 && m == 0 {
				if mode == "continue" {
					if err = closeScope(inner); err != nil {
						err = errors.Join(err, closeScope(outer))
						return
					}
					continue
				}
				if mode == "break" {
					if err = closeScope(inner); err != nil {
						err = errors.Join(err, closeScope(outer))
						return
					}
					break
				}
				if mode == "continue-outer" {
					err = errors.Join(closeScope(inner), closeScope(outer))
					if err != nil {
						return
					}
					continue outer
				}
				if mode == "break-outer" {
					err = errors.Join(closeScope(inner), closeScope(outer))
					if err != nil {
						return
					}
					break outer
				}
				if mode == "return" {
					// Evaluate the return expression before starting any cleanup.
					returned := append(visits, "return-value")
					err = errors.Join(closeScope(inner), closeScope(outer))
					return returned, err
				}
				if mode == "error" {
					err = errors.Join(errors.New("body failure"), closeScope(inner), closeScope(outer))
					return
				}
			}
			if err = closeScope(inner); err != nil {
				err = errors.Join(err, closeScope(outer))
				return
			}
		}
		visits = append(visits, "after-inner")
		if err = closeScope(outer); err != nil {
			return
		}
	}
	return append(visits, "after-outer"), nil
}

type scopeRecord struct {
	Scope string
	Event struct{ Kind, Error string }
}

func scopeEvents(t *testing.T, a *Activities) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(a.project.Dir(), "runs", "*", "run.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	d := json.NewDecoder(f)
	for d.More() {
		var r scopeRecord
		if err = d.Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.Scope != "" && (r.Event.Kind == "scope_began" || r.Event.Kind == "scope_ended") {
			out = append(out, r.Event.Kind+":"+r.Scope+":"+r.Event.Error)
		}
	}
	return out
}
func TestExplicitCleanupMatchesGoExits(t *testing.T) {
	expected := map[string][]string{
		"normal":         {"0/0", "0/1", "after-inner", "1/0", "1/1", "after-inner", "after-outer"},
		"continue":       {"0/0", "0/1", "after-inner", "1/0", "1/1", "after-inner", "after-outer"},
		"break":          {"0/0", "after-inner", "1/0", "1/1", "after-inner", "after-outer"},
		"continue-outer": {"0/0", "1/0", "1/1", "after-inner", "after-outer"},
		"break-outer":    {"0/0", "after-outer"}, "return": {"0/0", "return-value"}, "error": {"0/0"},
	}
	for mode, want := range expected {
		t.Run(mode, func(t *testing.T) {
			source := testActivities(t, "source-exits")
			got, sourceErr := sourceExits(source.scopes[""].ctx, mode)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("source %v want %v", got, want)
			}
			lowered := testActivities(t, "lowered-exits")
			var suite testsuite.WorkflowTestSuite
			e := suite.NewTestWorkflowEnvironment()
			e.RegisterActivity(lowered)
			e.ExecuteWorkflow(loweredExits, mode)
			if (e.GetWorkflowError() != nil) != (sourceErr != nil) {
				t.Fatal(e.GetWorkflowError(), sourceErr)
			}
			if sourceErr == nil {
				var actual []string
				if err := e.GetWorkflowResult(&actual); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, want) {
					t.Fatalf("target %v want %v", actual, want)
				}
			}
			if a, b := scopeEvents(t, source), scopeEvents(t, lowered); !reflect.DeepEqual(a, b) {
				t.Fatalf("source/target lifecycle differs:\n%v\n%v", a, b)
			}
			if len(lowered.scopes) != 1 {
				t.Fatal("leaked scope", lowered.scopes)
			}
		})
	}
}

func TestHelperReturnResumesCaller(t *testing.T) {
	source := testActivities(t, "helper-source")
	want, err := sourceExits(source.scopes[""].ctx, "return")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, "caller-resumed")
	target := testActivities(t, "helper-target")
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestWorkflowEnvironment()
	e.RegisterActivity(target)
	e.ExecuteWorkflow(func(ctx workflow.Context) ([]string, error) {
		v, err := loweredExits(ctx, "return")
		if err != nil {
			return nil, err
		}
		return append(v, "caller-resumed"), nil
	})
	var got []string
	if err = e.GetWorkflowResult(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
}
func TestCaughtScopeErrorLeavesParentUsable(t *testing.T) {
	source := testActivities(t, "caught-source")
	err := gimbal.Scope(source.scopes[""].ctx, "child", func(context.Context) error { return errors.New("caught") })
	if err == nil {
		t.Fatal("missing source error")
	}
	target := testActivities(t, "caught-target")
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestWorkflowEnvironment()
	e.RegisterActivity(target)
	e.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
		if err := workflow.ExecuteActivity(ctx, "EnterScope", ScopeInput{"child.1", "", "child"}).Get(ctx, nil); err != nil {
			return err
		}
		bodyErr := errors.New("caught")
		cleanup := cleanupContext(ctx)
		closeErr := workflow.ExecuteActivity(cleanup, "ExitScope", "child.1", bodyErr.Error()).Get(cleanup, nil)
		// Authored recovery catches the body failure, not a failure of cleanup.
		return closeErr
	})
	if err = e.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if target.scopes[""].ctx.Err() != nil {
		t.Fatal("caught child failure cancelled parent")
	}
	if a, b := scopeEvents(t, source), scopeEvents(t, target); !reflect.DeepEqual(a, b) {
		t.Fatal(a, b)
	}
}
func TestBodyAndSessionCleanupFailuresBothSurvive(t *testing.T) {
	a := newTestActivities(t)
	adapter := &specimenAdapter{closeErr: errors.New("close failed"), turn: func(context.Context, string, string) (any, error) { return Report{}, nil }}
	a.models = map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: "test"}}
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	if _, err := e.ExecuteActivity(a.Initialize, Data{Name: "cleanup-failure"}); err != nil {
		t.Fatal(err)
	}
	encoded, err := e.ExecuteActivity(a.NewParent)
	if err != nil {
		t.Fatal(err)
	}
	var handle sessionHandle
	if err = encoded.Get(&handle); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ExecuteActivity(a.ContinuityGenerate1, operationInput{Session: handle}); err != nil {
		t.Fatal(err)
	}
	_, err = e.ExecuteActivity(a.Finish, "body failed")
	if err == nil || !strings.Contains(err.Error(), "close failed") {
		t.Fatal("cleanup failure lost", err)
	}
	files, _ := filepath.Glob(filepath.Join(a.project.Dir(), "runs", "*", "run.jsonl"))
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "body failed") || !strings.Contains(string(raw), "close failed") {
		t.Fatal("combined failure not recorded")
	}
}
