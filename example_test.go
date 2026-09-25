package gimbal_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tylergannon/gimbal"
)

type exampleAdapter struct{}

func (*exampleAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "example-session", nil
}

func (a *exampleAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, emit func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	out, err := json.Marshal("done")
	return gimbal.TurnResult{Output: out}, err
}

func (*exampleAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*exampleAdapter) Fork(context.Context, string) (string, error) {
	return "example-fork", nil
}
func (*exampleAdapter) Close(context.Context, string) error { return nil }

func exampleContext() (context.Context, func()) {
	dir, err := os.MkdirTemp("", "gimbal-example-")
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return gimbal.Project(ctx, dir), func() {
		cancel()
		_ = os.RemoveAll(dir)
	}
}

func Example() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimbal.Run(ctx, "example", map[gimbal.WorkflowRole]gimbal.ModelBinding{"worker": {Adapter: &exampleAdapter{}, Model: "example"}}, func(ctx context.Context) error {
		gimbal.Set(ctx, "goal", "demonstrate the public API")
		worker := gimbal.NewSession(ctx, "worker", ".")
		answer, err := worker.Generate[gimbal.Text](ctx, "Complete the goal.")
		if err != nil {
			return err
		}
		fmt.Println(answer)
		return nil
	})
	fmt.Println(err)

	// Output:
	// done
	// <nil>
}

func ExampleGroup() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimbal.Run(ctx, "parallel", nil, func(ctx context.Context) error {
		group := gimbal.Group(ctx, "drafts")
		group.Go("draft", func(ctx context.Context) error {
			gimbal.Set(ctx, "approach", "first")
			return nil
		})
		group.Go("draft", func(ctx context.Context) error {
			gimbal.Set(ctx, "approach", "second")
			return nil
		})
		return group.Wait()
	})
	fmt.Println(err)

	// Output: <nil>
}

type exampleLoopAdapter struct{ turns int }

func (*exampleLoopAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "example-planner", nil
}

func (a *exampleLoopAdapter) RunTurn(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.turns++
	if a.turns > 1 {
		return gimbal.TurnResult{Output: json.RawMessage(`{"tasks":[],"next":null}`)}, nil
	}
	return gimbal.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"Show the task","description":"Make the structured assignment visible to the workflow.","definition_of_done":"The workflow receives and records the assignment.","validation":{"command":"","query":""}}],"next":0}`)}, nil
}

func (*exampleLoopAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*exampleLoopAdapter) Fork(context.Context, string) (string, error) {
	return "example-planner-fork", nil
}
func (*exampleLoopAdapter) Close(context.Context, string) error { return nil }

func ExamplePromiseLoop() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimbal.Run(ctx, "dispatch", map[gimbal.WorkflowRole]gimbal.ModelBinding{"planner": {Adapter: &exampleLoopAdapter{}, Model: "example"}}, func(ctx context.Context) error {
		planner := gimbal.NewSession(ctx, "planner", ".")
		loop := gimbal.PromiseLoop(ctx, "work", "demonstrate adaptive dispatch", planner)
		for ctx, task := range loop.Tasks {
			fmt.Println(task.Name)
			gimbal.Set(ctx, "result", "assignment recorded")
		}
		return loop.Err()
	})
	fmt.Println(err)

	// Output:
	// Show the task
	// <nil>
}

func ExampleIterate() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimbal.Run(ctx, "iterations", nil, func(ctx context.Context) error {
		count := 0
		for itemCtx, item := range gimbal.Iterate(ctx, "work", []string{"one", "two"}) {
			count++
			gimbal.Set(itemCtx, "item", item)
		}
		fmt.Println(count)
		return nil
	})
	fmt.Println(err)

	// Output:
	// 2
	// <nil>
}
