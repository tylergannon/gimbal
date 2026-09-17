package gimble_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tylergannon/gimble"
)

type exampleAdapter struct{}

func (*exampleAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "example-session", nil
}

func (a *exampleAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, emit func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	out, err := json.Marshal("done")
	return gimble.TurnResult{Output: out}, err
}

func (*exampleAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*exampleAdapter) Fork(context.Context, string) (string, error) {
	return "example-fork", nil
}
func (*exampleAdapter) Close(context.Context, string) error { return nil }

func exampleContext() (context.Context, func()) {
	dir, err := os.MkdirTemp("", "gimble-example-")
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return gimble.Project(ctx, dir), func() {
		cancel()
		_ = os.RemoveAll(dir)
	}
}

func Example() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimble.Run(ctx, "example", map[string]gimble.ModelBinding{"worker": {Adapter: &exampleAdapter{}, Model: "example"}}, func(ctx context.Context) error {
		gimble.Set(ctx, "goal", "demonstrate the public API")
		worker := gimble.NewSession(ctx, "worker", ".")
		answer, err := worker.Generate[gimble.Text](ctx, "Complete the goal.")
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

	err := gimble.Run(ctx, "parallel", nil, func(ctx context.Context) error {
		group := gimble.Group(ctx, "drafts")
		group.Go("draft", func(ctx context.Context) error {
			gimble.Set(ctx, "approach", "first")
			return nil
		})
		group.Go("draft", func(ctx context.Context) error {
			gimble.Set(ctx, "approach", "second")
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

func (a *exampleLoopAdapter) RunTurn(_ context.Context, _ string, _ string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.turns++
	if a.turns > 1 {
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[],"next":null}`)}, nil
	}
	return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"Show the task","description":"Make the structured assignment visible to the workflow.","definition_of_done":"The workflow receives and records the assignment.","validation":{"command":"","query":""}}],"next":0}`)}, nil
}

func (*exampleLoopAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*exampleLoopAdapter) Fork(context.Context, string) (string, error) {
	return "example-planner-fork", nil
}
func (*exampleLoopAdapter) Close(context.Context, string) error { return nil }

func ExampleLoop() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimble.Run(ctx, "dispatch", map[string]gimble.ModelBinding{"planner": {Adapter: &exampleLoopAdapter{}, Model: "example"}}, func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", ".")
		loop := gimble.Loop(ctx, "work")
		for ctx, task := range loop.Tasks("demonstrate adaptive dispatch", planner) {
			fmt.Println(task.Name)
			gimble.Set(ctx, "result", "assignment recorded")
		}
		return loop.Err()
	})
	fmt.Println(err)

	// Output:
	// Show the task
	// <nil>
}

func ExampleLoop_iterations() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimble.Run(ctx, "iterations", nil, func(ctx context.Context) error {
		loop := gimble.Loop(ctx, "work")
		count := 0
		for iterationCtx := range loop.Iterations {
			count++
			gimble.Set(iterationCtx, "item", count)
			if count == 2 {
				break
			}
		}
		fmt.Println(count, loop.Err())
		return loop.Err()
	})
	fmt.Println(err)

	// Output:
	// 2 <nil>
	// <nil>
}
