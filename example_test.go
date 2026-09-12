package gimble_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tylergannon/gimble"
)

type exampleAdapter struct{}

func (*exampleAdapter) CreateSession(context.Context, string, string) (string, error) {
	return "example-session", nil
}

func (a *exampleAdapter) RunTurn(_ context.Context, _ string, prompt string, schema json.RawMessage, emit func(gimble.AgentEvent) error) (json.RawMessage, error) {
	return json.Marshal("done")
}

func (*exampleAdapter) Steer(context.Context, string, string) error { return nil }
func (*exampleAdapter) Fork(context.Context, string) (string, error) {
	return "example-fork", nil
}

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

	err := gimble.Run(ctx, "example", func(ctx context.Context) error {
		if err := gimble.Set(ctx, "goal", "demonstrate the public API"); err != nil {
			return err
		}
		worker := gimble.NewSession(ctx, "worker", &exampleAdapter{}, "example", ".")
		answer, err := worker.Generate[gimble.Text](ctx,
			"Complete the goal.\n\n"+gimble.ScopeText(ctx))
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

	err := gimble.Run(ctx, "parallel", func(ctx context.Context) error {
		group := gimble.Group(ctx, "drafts")
		group.Go("draft", func(ctx context.Context) error {
			return gimble.Set(ctx, "approach", "first")
		})
		group.Go("draft", func(ctx context.Context) error {
			return gimble.Set(ctx, "approach", "second")
		})
		return group.Wait()
	})
	fmt.Println(err)

	// Output: <nil>
}

type exampleLoopAdapter struct{ turns int }

func (*exampleLoopAdapter) CreateSession(context.Context, string, string) (string, error) {
	return "example-planner", nil
}

func (a *exampleLoopAdapter) RunTurn(_ context.Context, _ string, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent)) (json.RawMessage, error) {
	a.turns++
	if a.turns > 1 {
		return json.RawMessage(`{"next":null}`), nil
	}
	marker := "Its revisable backlog is "
	rest := prompt[strings.Index(prompt, marker)+len(marker):]
	file := strings.TrimSuffix(strings.Fields(rest)[0], ".")
	backlog := `---
goal: demonstrate adaptive dispatch
tasks:
  - name: Show the task
    description: Make the structured assignment visible to the workflow.
    definition_of_done: The workflow receives and records the assignment.
    validation: {}
---
`
	if err := os.WriteFile(file, []byte(backlog), 0o644); err != nil {
		return nil, err
	}
	return json.RawMessage(`{"next":{"name":"Show the task","description":"Make the structured assignment visible to the workflow.","definition_of_done":"The workflow receives and records the assignment.","validation":{"command":"","query":""}}}`), nil
}

func (*exampleLoopAdapter) Steer(context.Context, string, string) error { return nil }
func (*exampleLoopAdapter) Fork(context.Context, string) (string, error) {
	return "example-planner-fork", nil
}

func ExampleLoop() {
	ctx, closeProject := exampleContext()
	defer closeProject()

	err := gimble.Run(ctx, "dispatch", func(ctx context.Context) error {
		planner := gimble.NewSession(ctx, "planner", &exampleLoopAdapter{}, "example", ".")
		loop := gimble.Loop(ctx, "work", "demonstrate adaptive dispatch", planner)
		for ctx, task := range loop.Tasks {
			fmt.Println(task.Name)
			if err := gimble.Set(ctx, "result", "assignment recorded"); err != nil {
				return err
			}
		}
		return loop.Err()
	})
	fmt.Println(err)

	// Output:
	// Show the task
	// <nil>
}
