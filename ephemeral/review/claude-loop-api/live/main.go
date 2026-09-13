// A review workflow written the way a first-time author would write it, to
// judge how the primitives feel. Codex gpt-5.6-luna plans and codes; Claude
// Haiku validates independently; an external acceptance program is the
// deterministic gate. Run from the repository root:
//
//	go run ./ephemeral/review/claude-loop-api/live
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/web"
)

const (
	planModel   = "gpt-5.6-luna"
	reviewModel = "haiku"
	taskBudget  = 4
)

func main() {
	base, _ := filepath.Abs("ephemeral/review/claude-loop-api")
	workspace := filepath.Join(base, "workspace")
	acceptance := filepath.Join(base, "acceptance.py")
	spec, err := os.ReadFile(filepath.Join(workspace, "SPEC.md"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Minute)
	defer cancel()
	runtime, err := web.NewRuntime(ctx, filepath.Join(base, "project"), web.WithPort(18091))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("models: planner/worker", planModel, "validator", reviewModel)
	cx, cl := codex.New(), claude.New()

	err = runtime.Run(ctx, "semverbump", func(ctx context.Context) error {
		// Root scope: what every agent in the run should know.
		if err := gimble.Set(ctx, "specification", string(spec)); err != nil {
			return err
		}
		if err := gimble.Set(ctx, "constraints", "Standard library only. Work only in the workspace. Do not commit. The acceptance program is external and cannot be changed; its output is authoritative."); err != nil {
			return err
		}
		if err := gimble.Set(ctx, "role", "You plan; you do not implement."); err != nil {
			return err
		}
		initial, _ := run(ctx, workspace, "python3", acceptance, workspace)
		if err := gimble.Set(ctx, "initial acceptance", initial); err != nil {
			return err
		}

		planner := gimble.NewSession(ctx, "planner", cx, planModel, workspace)
		loop := gimble.Loop(ctx, "delivery", "Deliver SPEC.md so that the external acceptance program passes completely, with tests and a README.", planner)
		n := 0
		for ctx, task := range loop.Tasks {
			if n++; n > taskBudget {
				fmt.Println("budget reached; stopping dispatch")
				break
			}
			fmt.Printf("task %d: %s\n", n, task.Name)
			if err := gimble.Set(ctx, "role", "You implement the assignment in the workspace and demonstrate it."); err != nil {
				return err
			}
			worker := gimble.NewSession(ctx, "worker", cx, planModel, workspace)
			result, err := worker.Generate[gimble.Text](ctx, "Complete the assignment.\n\n"+gimble.ScopeText(ctx))
			if err != nil {
				return err
			}
			if err := gimble.Set(ctx, "worker result", string(result)); err != nil {
				return err
			}

			// Validation: the deterministic gate and an independent reader, at once.
			var accepted, verdict string
			var code int
			g := gimble.Group(ctx, "validation")
			g.Go("acceptance", func(ctx context.Context) error {
				accepted, code = run(ctx, workspace, "python3", acceptance, workspace)
				return ctx.Err()
			})
			g.Go("review", func(ctx context.Context) error {
				if err := gimble.Set(ctx, "role", "You are a read-only validator. Change nothing."); err != nil {
					return err
				}
				reviewer := gimble.NewSession(ctx, "reviewer", cl, reviewModel, workspace)
				answer, err := reviewer.Generate[gimble.Text](ctx, "Read SPEC.md, the implementation, the tests and the README. Answer PASS, or FAIL followed by each unmet requirement you can demonstrate. The acceptance program runs separately; do not run it.\n\n"+gimble.ScopeText(ctx))
				verdict = string(answer)
				return err
			})
			if err := g.Wait(); err != nil {
				return err
			}
			// Group children are scopes of their own, so hand the planner what it needs.
			if err := gimble.Set(ctx, "acceptance", accepted); err != nil {
				return err
			}
			if err := gimble.Set(ctx, "independent review", verdict); err != nil {
				return err
			}
			fmt.Printf("task %d acceptance exit %d\n%s\nreview: %s\n", n, code, lastLine(accepted), firstLine(verdict))
		}
		if err := loop.Err(); err != nil {
			return err
		}

		// The planner stopping is not proof. The gate is.
		final, code := run(ctx, workspace, "python3", acceptance, workspace)
		if err := gimble.Set(ctx, "final acceptance", final); err != nil {
			return err
		}
		fmt.Println("final:", lastLine(final))
		if code != 0 {
			return errors.New("goal unmet: acceptance failed after dispatch ended")
		}
		return nil
	})
	fmt.Println("run error:", err)
	_ = os.WriteFile(filepath.Join(base, "live", "finished.txt"), []byte(fmt.Sprintf("%v\n", err)), 0o644)
}

func run(ctx context.Context, dir string, args ...string) (string, int) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		code = -1
	}
	return fmt.Sprintf("$ %s\nexit %d\n%s", strings.Join(args, " "), code, out), code
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if len(line) > 160 {
		line = line[:160] + "..."
	}
	return line
}
