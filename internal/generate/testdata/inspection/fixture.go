// Package inspection exercises nested context and scope semantics without a run.
package inspection

import (
	"context"
	"encoding/json"
	"github.com/tylergannon/gimbal"
)

type Context struct {
	Product struct {
		Name   string  `json:"name"`
		Guides []Guide `json:"guides,omitempty"`
	} `json:"product"`
	Next    *Context `json:"next,omitempty"`
	Ignored string   `json:"-"`
}
type Guide struct {
	Path     string   `json:"path"`
	Sections []string `json:"sections"`
}

func (Context) Schema() json.RawMessage   { return nil }
func (Context) ValidateJSON([]byte) error { return nil }

const initialPrompt = "Read the product context.\nDescribe its nested shape."

func Inspect(ctx context.Context, prompt string, conditional bool, key string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	gimbal.Set(ctx, "shared", "parent")
	gimbal.SetJSON(ctx, "brief", Context{})
	s := gimbal.NewSession(ctx, "reviewer", ".")
	_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	gimbal.Set(ctx, "later", true)
	group := gimbal.Group(ctx, "parallel")
	group.Go("left", func(ctx context.Context) error {
		gimbal.Set(ctx, "shared", 42)
		if conditional {
			gimbal.Set(ctx, "optional", "present")
		}
		_, _ = s.Generate[gimbal.Text](ctx, prompt)
		return nil
	})
	group.Go("right", func(ctx context.Context) error {
		gimbal.Set(ctx, "sibling", "right only")
		_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
		return nil
	})
	_ = group.Wait()
	_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	gimbal.Set(ctx, key, "dynamic")
	_, _ = s.Generate[gimbal.Text](ctx, prompt+"!")
	return nil
}

func Switches(ctx context.Context, mode string) error {
	s := gimbal.NewSession(ctx, "reviewer", ".")
	switch mode {
	case "review":
		_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	}
	switch mode {
	case "stop":
		return nil
	}
	_, _ = s.Generate[gimbal.Text](ctx, initialPrompt)
	return nil
}

// Routing includes ordered alternatives, nested scopes, and a switch.
// It is read by the source inspector and is never executed by generation.
func Routing(ctx context.Context, start, existing, mode string, checks []string) error {
	// Review brief
	// Keep the product and requested review together in context.
	gimbal.SetJSON(ctx, "brief", Context{})
	gimbal.Set(ctx, "review mode", mode)
	// Product reviewer
	reviewer := gimbal.NewSession(ctx, "reviewer", ".")

	// Start command supplied?
	// Start a new product when a command was supplied.
	// When true: Command provided
	if start != "" {
		// Start the product
		if err := gimbal.Service(ctx, "product", ".", start); err != nil {
			return err
		}
		// Check the started product
		// Each requested check runs against the newly started product.
		for _, check := range checks {
			gimbal.Set(ctx, "check", check)
			// Review one check
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Review the current check against the product.")
		}
	} else
	// Existing product supplied?
	// Use the supplied location when no start command was provided.
	// When true: Product location provided
	if existing != "" {
		// Inspect the existing product
		// Keep connection details local to this review.
		_ = gimbal.Scope(ctx, "existing-product", func(ctx context.Context) error {
			gimbal.Set(ctx, "product location", existing)
			// Confirm access
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Inspect the supplied product location and report whether it is accessible.")
			return nil
		})
	} else {
		// Review the setup instructions
		// Without a running product, inspect the instructions instead.
		_ = gimbal.Scope(ctx, "setup-review", func(ctx context.Context) error {
			gimbal.Set(ctx, "review target", "setup instructions")
			// Find missing setup steps
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Review the setup instructions for missing prerequisites and unclear steps.")
			return nil
		})
	}

	// Choose the review
	// The requested mode selects one review path.
	switch mode {
	// Visual review
	case "visual":
		// Review each screen
		for _, check := range checks {
			gimbal.Set(ctx, "screen", check)
			// Inspect the screen
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Review the current screen for readability and hierarchy.")
		}
	// Functional review
	case "functional":
		// Independent checks
		group := gimbal.Group(ctx, "functional-checks")
		// Main task
		group.Go("main-task", func(ctx context.Context) error {
			gimbal.Set(ctx, "task", "primary user task")
			// Try the primary task
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Try the primary task and report obstacles.")
			return nil
		})
		// Recovery path
		group.Go("recovery", func(ctx context.Context) error {
			gimbal.Set(ctx, "task", "error recovery")
			// Try recovery
			_, _ = reviewer.Generate[gimbal.Text](ctx, "Try the error recovery path and report obstacles.")
			return nil
		})
		_ = group.Wait()
	default:
		// Review the brief
		_, _ = reviewer.Generate[gimbal.Text](ctx, initialPrompt)
	}
	// Summarize the findings
	_, _ = reviewer.Generate[gimbal.Text](ctx, "Summarize the findings from the selected review.")
	return nil
}
