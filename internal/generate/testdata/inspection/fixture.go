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
