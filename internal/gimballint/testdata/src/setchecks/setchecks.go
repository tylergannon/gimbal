package setchecks

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

const namedKey = "named"
const joinedKey = "joined-" + "key"

func keyFromFunction() string { return "function" }

func keyChecks(ctx context.Context, input string, value any) {
	gimbal.Set(ctx, "literal", value)
	gimbal.Set(ctx, namedKey, value)
	gimbal.SetJSON(ctx, joinedKey, value)
	_ = gimbal.Check(ctx, "check", ".", "true")
	gimbal.Set(ctx, "dynamic value", input)

	variable := "variable"
	gimbal.Set(ctx, variable, value)                           // want `GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimbal.Set(ctx, input, value)                              // want `GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimbal.Set(ctx, fmt.Sprintf("formatted %s", input), value) // want `GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimbal.SetJSON(ctx, keyFromFunction(), value)              // want `GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	_ = gimbal.Check(ctx, input, ".", "true")                  // want `GIMBAL102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
}

func duplicateChecks(ctx context.Context, branch bool) {
	gimbal.Set(ctx, "duplicate", 1)
	gimbal.SetJSON(ctx, "duplicate", 2) // want `GIMBAL103-SET-MISUSE/DUPLICATE-KEY`
	_ = gimbal.Check(ctx, "check", ".", "true")
	_ = gimbal.Check(ctx, "check", ".", "true") // want `GIMBAL103-SET-MISUSE/DUPLICATE-KEY`

	if branch {
		gimbal.Set(ctx, "one branch", 1)
	} else {
		gimbal.Set(ctx, "one branch", 2)
	}

	for i := 0; i < 2; i++ {
		gimbal.Set(ctx, "loop duplicate", i) // want `GIMBAL103-SET-MISUSE/DUPLICATE-KEY`
	}
}

func boundaryChecks(parent context.Context) error {
	return gimbal.Run(parent, "root", func(ctx context.Context) error {
		gimbal.Set(ctx, "own root", 1)
		if err := gimbal.Scope(ctx, "child", func(child context.Context) error {
			gimbal.Set(child, "own child", 1)
			gimbal.Set(ctx, "parent write", 2) // want `GIMBAL104-SET-MISUSE/WRONG-CONTEXT`
			go func() {
				gimbal.Set(child, "async", 3) // want `GIMBAL105-SET-MISUSE/UNJOINED-GOROUTINE`
			}()
			return nil
		}); err != nil {
			return err
		}

		group := gimbal.Group(ctx, "joined")
		group.Go("child", func(child context.Context) error {
			gimbal.Set(child, "joined", 1)
			return nil
		})
		return group.Wait()
	})
}

func contextChecks(ctx context.Context) {
	gimbal.Set(context.Background(), "background", 1) // want `GIMBAL107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE`
	gimbal.SetJSON(context.TODO(), "todo", 1)         // want `GIMBAL107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE`
	gimbal.Set(ctx, "scope", 1)
}

func taskChecks(ctx context.Context) {
	loop := gimbal.PromiseLoop(ctx, "work", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		gimbal.Set(taskCtx, "task", task) // want `GIMBAL106-SET-MISUSE/RESERVED-TASK-KEY`
		gimbal.Set(taskCtx, "result", task)
		gimbal.Set(ctx, "outer loop context", task) // want `GIMBAL103-SET-MISUSE/DUPLICATE-KEY` `GIMBAL104-SET-MISUSE/WRONG-CONTEXT`
	}
}

func iterationChecks(ctx context.Context) {
	for iterationCtx := range gimbal.Iterate(ctx, "iterations", []int{1}) {
		gimbal.Set(iterationCtx, "task", 1)
		gimbal.Set(ctx, "outer iteration context", 1) // want `GIMBAL103-SET-MISUSE/DUPLICATE-KEY` `GIMBAL104-SET-MISUSE/WRONG-CONTEXT`
	}
}
