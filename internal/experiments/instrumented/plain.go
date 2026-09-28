// Package main is a hand-written specimen of Temporal compiler output.
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry Plain -name instrumented

const coder gimbal.WorkflowRole = "coder"
const coach gimbal.WorkflowRole = "coach"
const repairPrompt = "Read the complete reference context file, find the line beginning CONTEXT_RECEIPT= in its middle, and return its value as receipt. Fix the defect described in assignment. Read the assigned source and its tests, edit only the assigned source file, and run the assigned test. Other agents share this directory: do not edit their files, tests, or go.mod. Use the previous iteration's checks as context. Return a concise summary and the name of the file you changed."
const coachPrompt = "Keep the worker within its assigned file and requested fix. Object to edits of tests, other workers' files, or unrelated functionality."

type Params struct{ Task string }
type Checks struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}
type Report struct {
	Summary string `json:"summary"`
	File    string `json:"file"`
	Receipt string `json:"receipt"`
}
type Assignment struct {
	File string `json:"file"`
	Task string `json:"task"`
	Test string `json:"test"`
}
type Pair struct {
	Number      int
	Left, Right Assignment
	Test        string
}

var pairs = []Pair{
	{1, Assignment{"add.go", "Add must add two integers.", "TestAdd"}, Assignment{"multiply.go", "Multiply must multiply two integers.", "TestMultiply"}, "TestAdd|TestMultiply"},
	{2, Assignment{"reverse.go", "Reverse must reverse Unicode characters, not bytes.", "TestReverse"}, Assignment{"clamp.go", "Clamp must return the nearest bound for values outside [low, high].", "TestClamp"}, "TestReverse|TestClamp"},
}

// Plain is ordinary Gimbal source; Temporal executes its handwritten counterpart.
func Plain(ctx context.Context, env gimbal.Env, in Params) error {
	gimbal.Set(ctx, "task", in.Task)
	if err := prepareFixture(env.WorkDir); err != nil {
		return err
	}
	previous := Checks{}
	for ctx, pair := range gimbal.Iterate(ctx, "pairs", pairs) {
		gimbal.Set(ctx, "iteration", fmt.Sprint(pair.Number))
		gimbal.SetJSON(ctx, "previous", previous)
		gimbal.Set(ctx, "layer", "iteration-layer")
		if err := gimbal.Scope(ctx, "context", func(ctx context.Context) error {
			gimbal.Set(ctx, "layer", "outer-layer")
			gimbal.Set(ctx, "inherited", "outer-inherited")
			gimbal.Set(ctx, "reference", referenceMaterial())
			gimbal.Set(ctx, "support-a", supportMaterial("a"))
			gimbal.Set(ctx, "support-b", supportMaterial("b"))
			gimbal.Set(ctx, "support-c", supportMaterial("c"))
			gimbal.Set(ctx, "support-d", supportMaterial("d"))
			gimbal.Set(ctx, "support-e", supportMaterial("e"))
			return gimbal.Scope(ctx, "details", func(ctx context.Context) error {
				gimbal.Set(ctx, "layer", "inner-layer")
				gimbal.Set(ctx, "child-only", "inner-private")
				group := gimbal.Group(ctx, "fixes")
				group.Go("left", func(ctx context.Context) error {
					gimbal.SetJSON(ctx, "assignment", pair.Left)
					principal := gimbal.NewSession(ctx, coder, env.WorkDir)
					supervisor := gimbal.NewSession(ctx, coach, env.WorkDir)
					_, err := principal.Generate[Report](ctx, repairPrompt, gimbal.WithSupervisor(supervisor, coachPrompt))
					return err
				})
				group.Go("right", func(ctx context.Context) error {
					gimbal.SetJSON(ctx, "assignment", pair.Right)
					principal := gimbal.NewSession(ctx, coder, env.WorkDir)
					supervisor := gimbal.NewSession(ctx, coach, env.WorkDir)
					_, err := principal.Generate[Report](ctx, repairPrompt, gimbal.WithSupervisor(supervisor, coachPrompt))
					return err
				})
				return group.Wait()
			})
		}); err != nil {
			return err
		}
		code, stdout, stderr, err := gimbal.RunCommand(ctx, "tests", env.WorkDir, "go", "test", "-count=1", "-run", pair.Test, "./...")
		if err != nil {
			return err
		}
		previous = Checks{code, stdout, stderr}
		if code != 0 {
			return fmt.Errorf("iteration checks exited %d: %s%s", code, stdout, stderr)
		}
	}
	code, stdout, stderr, err := gimbal.RunCommand(ctx, "final-tests", env.WorkDir, "go", "test", "-count=1", "./...")
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("final checks exited %d: %s%s", code, stdout, stderr)
	}
	return ctx.Err()
}

// Prepared context exercises transport and prompt budgeting without introducing
// another workflow branch or making agents synthesize megabytes of data.
func referenceMaterial() string {
	return strings.Repeat("Reference padding; no implementation instructions here.\n", 12000) + "CONTEXT_RECEIPT=middle-of-external-context-42\n" + strings.Repeat("Reference padding; no implementation instructions here.\n", 12000)
}
func supportMaterial(name string) string {
	return strings.Repeat("Supporting context "+name+"; retain complete data outside the prompt.\n", 1000)
}
