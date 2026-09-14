package setchecks

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

const namedKey = "named"
const joinedKey = "joined-" + "key"

func keyFromFunction() string { return "function" }

func keyChecks(ctx context.Context, input string, value any) {
	gimble.Set(ctx, "literal", value)
	gimble.Set(ctx, namedKey, value)
	gimble.SetJSON(ctx, joinedKey, value)
	gimble.Set(ctx, "dynamic value", input)

	variable := "variable"
	gimble.Set(ctx, variable, value)                           // want `GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimble.Set(ctx, input, value)                              // want `GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimble.Set(ctx, fmt.Sprintf("formatted %s", input), value) // want `GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
	gimble.SetJSON(ctx, keyFromFunction(), value)              // want `GIMBLE102-SIMPLE-WORKFLOWS/CONSTANT-CONTEXT-KEY`
}

func duplicateChecks(ctx context.Context, branch bool) {
	gimble.Set(ctx, "duplicate", 1)
	gimble.SetJSON(ctx, "duplicate", 2) // want `GIMBLE103-SET-MISUSE/DUPLICATE-KEY`

	if branch {
		gimble.Set(ctx, "one branch", 1)
	} else {
		gimble.Set(ctx, "one branch", 2)
	}

	for i := 0; i < 2; i++ {
		gimble.Set(ctx, "loop duplicate", i) // want `GIMBLE103-SET-MISUSE/DUPLICATE-KEY`
	}
}

func boundaryChecks(parent context.Context) error {
	return gimble.Run(parent, "root", func(ctx context.Context) error {
		gimble.Set(ctx, "own root", 1)
		if err := gimble.Scope(ctx, "child", func(child context.Context) error {
			gimble.Set(child, "own child", 1)
			gimble.Set(ctx, "parent write", 2) // want `GIMBLE104-SET-MISUSE/WRONG-CONTEXT`
			go func() {
				gimble.Set(child, "async", 3) // want `GIMBLE105-SET-MISUSE/UNJOINED-GOROUTINE`
			}()
			return nil
		}); err != nil {
			return err
		}

		group := gimble.Group(ctx, "joined")
		group.Go("child", func(child context.Context) error {
			gimble.Set(child, "joined", 1)
			return nil
		})
		return group.Wait()
	})
}

func contextChecks(ctx context.Context) {
	gimble.Set(context.Background(), "background", 1) // want `GIMBLE107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE`
	gimble.SetJSON(context.TODO(), "todo", 1)         // want `GIMBLE107-SET-MISUSE/CONTEXT-NOT-FROM-SCOPE`
	gimble.Set(ctx, "scope", 1)
}

func taskChecks(ctx context.Context) {
	loop := gimble.Loop(ctx, "work", "goal", nil)
	for taskCtx, task := range loop.Tasks {
		gimble.Set(taskCtx, "task", task) // want `GIMBLE106-SET-MISUSE/RESERVED-TASK-KEY`
		gimble.Set(taskCtx, "result", task)
		gimble.Set(ctx, "outer loop context", task) // want `GIMBLE103-SET-MISUSE/DUPLICATE-KEY` `GIMBLE104-SET-MISUSE/WRONG-CONTEXT`
	}
}
