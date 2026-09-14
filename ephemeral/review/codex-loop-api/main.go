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

func main() {
	base, _ := filepath.Abs("ephemeral/review/loop-api")
	dir := filepath.Join(base, "workspace")
	goal, err := os.ReadFile(filepath.Join(base, "goal.txt"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	runtime, err := web.NewRuntime(ctx, filepath.Join(base, "project"), web.WithPort(18089))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Models: planner/worker=gpt-5.6-luna; independent reviewer=haiku")
	fmt.Println("Monitor: http://127.0.0.1:18089")
	cx, cl := codex.New(), claude.New()
	err = runtime.Run(ctx, "mergequeue-review", func(ctx context.Context) error {
		gimble.Set(ctx, "requirements", string(goal))
		gimble.Set(ctx, "role", "Plan work; edit only the runtime backlog.")
		gimble.Set(ctx, "constraints", "Use only the standard library. Work in the isolated fixture. Do not commit or access GitHub. Acceptance is external and cannot be changed by agents.")
		initial, _, err := command(ctx, dir, "python3", filepath.Join(base, "acceptance.py"), dir)
		if err != nil {
			return err
		}
		gimble.Set(ctx, "initial acceptance", initial)
		planner := gimble.NewSession(ctx, "planner", cx, "gpt-5.6-luna", dir)
		loop := gimble.Loop(ctx, "delivery", "Deliver SPEC.md and satisfy recorded acceptance, including newly disclosed requirements. Stop when the implementation, examples, and tests are demonstrated.", planner)
		count := 0
		extended := false
		for taskCtx, task := range loop.Tasks {
			count++
			if count > 5 {
				return errors.New("task budget exceeded")
			}
			fmt.Printf("DISPATCH %d: %s\n", count, task.Name)
			gimble.Set(taskCtx, "role", "Implement the selected task and demonstrate it; leave changes uncommitted.")
			worker := gimble.NewSession(taskCtx, "worker", cx, "gpt-5.6-luna", dir)
			result, err := worker.Generate[gimble.Text](taskCtx, "Complete the assignment using this scoped context.\n\n"+gimble.ScopeText(taskCtx))
			if err != nil {
				return err
			}
			gimble.Set(taskCtx, "worker result", string(result))
			var acceptance, checks, assessment string
			var acceptanceCode int
			group := gimble.Group(taskCtx, "validation")
			group.Go("commands", func(checkCtx context.Context) error {
				var err error
				acceptance, acceptanceCode, err = command(checkCtx, dir, "python3", filepath.Join(base, "acceptance.py"), dir)
				if err != nil {
					return err
				}
				checks, _, err = command(checkCtx, dir, "sh", "-c", "go vet ./... && go test ./...")
				if err != nil {
					return err
				}
				gimble.Set(checkCtx, "command evidence", acceptance+"\n"+checks)
				return nil
			})
			group.Go("independent", func(reviewCtx context.Context) error {
				gimble.Set(reviewCtx, "role", "Read-only independent validator; assess the requested behavior, tests and README. Do not edit, commit, or access GitHub.")
				reviewer := gimble.NewSession(reviewCtx, "reviewer", cl, "haiku", dir)
				review, err := reviewer.Generate[gimble.Text](reviewCtx, "Inspect SPEC.md and the implementation. Identify only unmet requirements or invalid evidence. Reply PASS if no such defects remain, otherwise FAIL and concrete defects. Read-only source review: the workflow independently runs external black-box checks in parallel.\n\n"+gimble.ScopeText(reviewCtx))
				if err != nil {
					return err
				}
				assessment = string(review)
				gimble.Set(reviewCtx, "assessment", assessment)
				return nil
			})
			if err := group.Wait(); err != nil {
				return err
			}
			// Descendant scope values are not included in Loop's next prompt: promote selected results explicitly.
			gimble.Set(taskCtx, "acceptance evidence", acceptance)
			gimble.Set(taskCtx, "repository checks", checks)
			gimble.Set(taskCtx, "independent assessment", assessment)
			fmt.Printf("RESULT %d:\n%s\n%s\n%s\n", count, acceptance, checks, assessment)
			if !extended && acceptanceCode == 0 {
				extended = true
				newRequirement := "New requirement disclosed after the first successful base acceptance: add --parallel N to limit each wave to at most N ready PRs, choosing lowest IDs first. The default is unlimited. Reject missing, zero, negative, or non-integer N with exit 2 and no stdout. Document the flag."
				f, err := os.OpenFile(filepath.Join(dir, "SPEC.md"), os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				_, err = f.WriteString("\n" + newRequirement + "\n")
				f.Close()
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "parallel-requirement.txt"), []byte(newRequirement), 0644); err != nil {
					return err
				}
				next, _, err := command(taskCtx, dir, "python3", filepath.Join(base, "parallel.py"), dir)
				if err != nil {
					return err
				}
				gimble.Set(taskCtx, "new requirement", newRequirement)
				gimble.Set(taskCtx, "new requirement evidence", next)
				fmt.Println("EVOLVING REQUIREMENT:", next)
			} else if extended {
				next, _, err := command(taskCtx, dir, "python3", filepath.Join(base, "parallel.py"), dir)
				if err != nil {
					return err
				}
				gimble.Set(taskCtx, "new requirement evidence", next)
				fmt.Println("PARALLEL ACCEPTANCE:", next)
			}
		}
		if err := loop.Err(); err != nil {
			return err
		}
		// A stopped planner is not proof of fulfillment. Enforce final independent black-box gates.
		for _, file := range []string{"acceptance.py", "parallel.py"} {
			out, code, err := command(ctx, dir, "python3", filepath.Join(base, file), dir)
			if err != nil {
				return err
			}
			fmt.Println("FINAL:", out)
			gimble.Set(ctx, file, out)
			if code != 0 {
				return fmt.Errorf("goal unmet: %s", file)
			}
		}
		return nil
	})
	fmt.Printf("RUN ERROR: %v\n", err)
	os.WriteFile(filepath.Join(base, "finished.txt"), []byte(fmt.Sprintf("%v\n", err)), 0644)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("Serving result for 3 minutes")
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Minute):
	}
}
func command(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if ctx.Err() != nil {
		return "", 0, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return "", 0, err
		}
		code = exit.ExitCode()
	}
	return fmt.Sprintf("$ %s\nexit %d\n%s", strings.Join(args, " "), code, out), code, nil
}
