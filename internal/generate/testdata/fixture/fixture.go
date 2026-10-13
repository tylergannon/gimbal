// Package fixture is a workflow written for the extractor's tests: it holds
// one of each site the rules name, including the ones they cannot read.
package fixture

import (
	"context"

	"github.com/tylergannon/gimbal"
)

const (
	workPrompt       = "do the work"
	helperPrompt     = "check the work"
	watchInstruction = "watch the work"
	chiefInstruction = "watch the watcher"
)

// Fixture is the entry function the tests extract.
func Fixture(ctx context.Context, _ gimbal.Env) error {
	lead := gimbal.NewSession(ctx, "lead", ".")
	watcher := gimbal.NewSession(ctx, "watcher", ".")
	chief := gimbal.NewSession(ctx, "chief", ".")
	if _, err := lead.Generate[gimbal.Text](ctx, workPrompt,
		gimbal.WithSupervisor(watcher, watchInstruction,
			gimbal.WithSupervisor(chief, chiefInstruction),
		),
	); err != nil {
		return err
	}

	pair := gimbal.Group(ctx, "pair")
	pair.Go("left", func(ctx context.Context) error {
		gimbal.Set(ctx, "left", "one")
		return nil
	})
	pair.Go("right", func(ctx context.Context) error {
		gimbal.Set(ctx, "right", "two")
		return nil
	})
	if err := pair.Wait(); err != nil {
		return err
	}

	// A role the source does not spell out cannot be read.
	nameless := gimbal.NewSession(ctx, gimbal.WorkflowRole(roleName()), ".")
	_ = nameless

	// A call through a function value cannot be read either.
	speak := lead.Generate[gimbal.Text]
	if _, err := speak(ctx, workPrompt); err != nil {
		return err
	}

	// The helper's late return ends the helper, not this body, because an
	// operation follows the call.
	if err := review(ctx, lead); err != nil {
		return err
	}
	gimbal.Set(ctx, "after", "three")

	// A session bound in a branch is gone at the join, so the call after it
	// names no session the source declares.
	var chosen *gimbal.Session
	if roleName() == "reader" {
		chosen = gimbal.NewSession(ctx, "first", ".")
	} else {
		chosen = gimbal.NewSession(ctx, "second", ".")
	}
	if _, err := chosen.Generate[gimbal.Text](ctx, workPrompt); err != nil {
		return err
	}

	// A group is started in the body that declares it, so a Go inside a
	// branch is not read.
	held := gimbal.Group(ctx, "held")
	if roleName() == "reader" {
		held.Go("nested", func(ctx context.Context) error {
			gimbal.Set(ctx, "nested", "six")
			return nil
		})
	}
	if err := held.Wait(); err != nil {
		return err
	}

	// A group reassigned after its declaration is not read, and neither is
	// what is started on it afterwards.
	one := gimbal.Group(ctx, "one")
	two := gimbal.Group(ctx, "two")
	one = two
	one.Go("child", func(ctx context.Context) error {
		gimbal.Set(ctx, "child", "four")
		return nil
	})
	if err := two.Wait(); err != nil {
		return err
	}

	// A session reassigned inside a callback stays unread after it: the
	// callback ran, so its old name would be a guess.
	swapped := gimbal.NewSession(ctx, "swapped", ".")
	other := gimbal.NewSession(ctx, "other", ".")
	if err := gimbal.Scope(ctx, "swap", func(ctx context.Context) error {
		swapped = other
		return nil
	}); err != nil {
		return err
	}
	if _, err := swapped.Generate[gimbal.Text](ctx, workPrompt); err != nil {
		return err
	}

	// A short declaration that reuses an identifier reassigns it too.
	again := gimbal.NewSession(ctx, "again", ".")
	again, marker := gimbal.NewSession(ctx, "once more", "."), true
	_ = marker
	if _, err := again.Generate[gimbal.Text](ctx, workPrompt); err != nil {
		return err
	}

	// A Gimbal call written inside another call's arguments is not read.
	_ = gimbal.NewSession(ctx, "outer", workdirOf(gimbal.NewSession(ctx, "inner", ".")))

	// A callback is its own function, so its early return is shape even
	// when the callback is written in a helper.
	return guarded(ctx)
}

// SprintShape is a compact workflow used to exercise the main graph shapes.
func SprintShape(ctx context.Context, _ gimbal.Env) error {
	researcher := gimbal.NewSession(ctx, "researcher", ".")
	planner, err := researcher.Fork(ctx, "planner")
	if err != nil {
		return err
	}
	plannerWatch := gimbal.NewSession(ctx, "planner-watch", ".")
	for round := 0; round < 1; round++ {
		if err := gimbal.Scope(ctx, "round", func(ctx context.Context) error {
			if err := gimbal.Service(ctx, "preview", ".", "exec sleep 30"); err != nil {
				return err
			}
			if err := gimbal.Check(ctx, "tests", ".", "go", "test", "./..."); err != nil {
				return err
			}
			loop := gimbal.PromiseLoop(ctx, "sprint", "review the code", planner,
				gimbal.WithSupervisor(plannerWatch, watchInstruction),
			)
			for ctx, task := range loop.Tasks {
				_ = task
				coder, err := researcher.Fork(ctx, "coder")
				if err != nil {
					return err
				}
				if _, err := coder.Generate[gimbal.Text](ctx, workPrompt); err != nil {
					return err
				}
				if ctx.Err() == nil {
					_, _, _, err = gimbal.RunCommand(ctx, "git", ".", "git", "status")
					if err != nil {
						return err
					}
				}
			}
			return loop.Err()
		}); err != nil {
			return err
		}
	}
	return nil
}

// MissingEnv is invalid as a generated workflow entry.
func MissingEnv(ctx context.Context) error { return nil }

type WorkDirParams struct {
	WorkDir string
}

// HasWorkDirParams is invalid because WorkDir belongs to gimbal.Env.
func HasWorkDirParams(ctx context.Context, _ gimbal.Env, _ WorkDirParams) error { return nil }

func IterationShape(ctx context.Context, _ gimbal.Env) error {
	for ctx := range gimbal.Iterate(ctx, "iteration", []string{"one", "two"}) {
		session := gimbal.NewSession(ctx, "reviewer", ".")
		if _, err := session.Generate[gimbal.Text](ctx, workPrompt); err != nil {
			return err
		}
	}
	return nil
}

func ServiceOwnershipShape(ctx context.Context, _ gimbal.Env) error {
	if err := gimbal.Service(ctx, "root-db", ".", "exec sleep 30"); err != nil {
		return err
	}
	if err := gimbal.Scope(ctx, "backend", func(ctx context.Context) error {
		if err := gimbal.Service(ctx, "api", ".", "exec sleep 30"); err != nil {
			return err
		}
		_, _, _, err := gimbal.RunCommand(ctx, "build", ".", "go", "build", "./...")
		return err
	}); err != nil {
		return err
	}
	for ctx := range gimbal.Iterate(ctx, "iteration", []string{"one", "two"}) {
		if err := gimbal.Service(ctx, "fixture", ".", "exec sleep 30"); err != nil {
			return err
		}
		if err := gimbal.Check(ctx, "tests", ".", "go", "test", "./..."); err != nil {
			return err
		}
	}
	return nil
}

func PlannerReassignmentShape(ctx context.Context, _ gimbal.Env) error {
	planner := gimbal.NewSession(ctx, "planner", ".")
	loop := gimbal.PromiseLoop(ctx, "tasks", "review the code", planner)
	for ctx, task := range loop.Tasks {
		planner = gimbal.NewSession(ctx, "replacement", ".")
		_ = task
	}
	return nil
}

// guarded scopes a body whose first branch returns before it writes anything.
func guarded(ctx context.Context) error {
	return gimbal.Scope(ctx, "guarded", func(ctx context.Context) error {
		if skipped(ctx) {
			return nil
		}
		gimbal.Set(ctx, "guarded", "five")
		return nil
	})
}

func skipped(ctx context.Context) bool { return ctx.Err() != nil }

func workdirOf(*gimbal.Session) string { return "." }

// review generates once and then returns early, after it has produced shape.
func review(ctx context.Context, session *gimbal.Session) error {
	if _, err := session.Generate[gimbal.Text](ctx, helperPrompt); err != nil {
		return err
	}
	passed := ctx.Err() == nil
	if !passed {
		return nil
	}
	gimbal.Set(ctx, "review", "kept")
	return nil
}

func roleName() string { return "reader" }
