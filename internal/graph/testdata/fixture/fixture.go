// Package fixture is a workflow written for the extractor's tests: it holds
// one of each site the rules name, including the ones they cannot read.
package fixture

import (
	"context"

	"github.com/tylergannon/gimble"
)

const (
	workPrompt       = "do the work"
	helperPrompt     = "check the work"
	watchInstruction = "watch the work"
	chiefInstruction = "watch the watcher"
)

// Fixture is the entry function the tests extract.
func Fixture(ctx context.Context) error {
	lead := gimble.NewSession(ctx, "lead", ".")
	watcher := gimble.NewSession(ctx, "watcher", ".")
	chief := gimble.NewSession(ctx, "chief", ".")
	if _, err := lead.Generate[gimble.Text](ctx, workPrompt,
		gimble.WithSupervisor(watcher, watchInstruction,
			gimble.WithSupervisor(chief, chiefInstruction),
		),
	); err != nil {
		return err
	}

	pair := gimble.Group(ctx, "pair")
	pair.Go("left", func(ctx context.Context) error {
		gimble.Set(ctx, "left", "one")
		return nil
	})
	pair.Go("right", func(ctx context.Context) error {
		gimble.Set(ctx, "right", "two")
		return nil
	})
	if err := pair.Wait(); err != nil {
		return err
	}

	// A role the source does not spell out cannot be read.
	nameless := gimble.NewSession(ctx, roleName(), ".")
	_ = nameless

	// A call through a function value cannot be read either.
	speak := lead.Generate[gimble.Text]
	if _, err := speak(ctx, workPrompt); err != nil {
		return err
	}

	// The helper's late return ends the helper, not this body, because an
	// operation follows the call.
	if err := review(ctx, lead); err != nil {
		return err
	}
	gimble.Set(ctx, "after", "three")

	// A session bound in a branch is gone at the join, so the call after it
	// names no session the source declares.
	var chosen *gimble.Session
	if roleName() == "reader" {
		chosen = gimble.NewSession(ctx, "first", ".")
	} else {
		chosen = gimble.NewSession(ctx, "second", ".")
	}
	if _, err := chosen.Generate[gimble.Text](ctx, workPrompt); err != nil {
		return err
	}

	// A group is started in the body that declares it, so a Go inside a
	// branch is not read.
	held := gimble.Group(ctx, "held")
	if roleName() == "reader" {
		held.Go("nested", func(ctx context.Context) error {
			gimble.Set(ctx, "nested", "six")
			return nil
		})
	}
	if err := held.Wait(); err != nil {
		return err
	}

	// A group reassigned after its declaration is not read, and neither is
	// what is started on it afterwards.
	one := gimble.Group(ctx, "one")
	two := gimble.Group(ctx, "two")
	one = two
	one.Go("child", func(ctx context.Context) error {
		gimble.Set(ctx, "child", "four")
		return nil
	})
	if err := two.Wait(); err != nil {
		return err
	}

	// A Gimble call written inside another call's arguments is not read.
	_ = gimble.NewSession(ctx, "outer", workdirOf(gimble.NewSession(ctx, "inner", ".")))

	// A callback is its own function, so its early return is shape even
	// when the callback is written in a helper.
	return guarded(ctx)
}

// guarded scopes a body whose first branch returns before it writes anything.
func guarded(ctx context.Context) error {
	return gimble.Scope(ctx, "guarded", func(ctx context.Context) error {
		if skipped(ctx) {
			return nil
		}
		gimble.Set(ctx, "guarded", "five")
		return nil
	})
}

func skipped(ctx context.Context) bool { return ctx.Err() != nil }

func workdirOf(*gimble.Session) string { return "." }

// review generates once and then returns early, after it has produced shape.
func review(ctx context.Context, session *gimble.Session) error {
	if _, err := session.Generate[gimble.Text](ctx, helperPrompt); err != nil {
		return err
	}
	passed := ctx.Err() == nil
	if !passed {
		return nil
	}
	gimble.Set(ctx, "review", "kept")
	return nil
}

func roleName() string { return "reader" }
