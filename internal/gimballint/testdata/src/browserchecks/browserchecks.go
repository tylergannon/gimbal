package browserchecks

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

const prompt = "Use the browser."

var session = &gimbal.Session{}
var reviewer = &gimbal.Session{}

type Params struct{}

// The review's example, verbatim except for openBrowser's missing dir
// argument: a helper's result is still a browser.

func openBrowser(ctx context.Context, dir string) (*gimbal.Browser, error) {
	return gimbal.NewBrowser(ctx, "browser", dir, "") // allowed: returned
}

func Workflow(ctx context.Context, _ gimbal.Env, _ Params) error {
	var kept *gimbal.Browser
	later := gimbal.Group(ctx, "later")
	err := gimbal.Scope(ctx, "owner", func(ctx context.Context) error {
		b, err := openBrowser(ctx, ".") // not assigned from NewBrowser, not a param, not an alias
		if err != nil {
			return err
		}
		kept = b                                          // want `GIMBAL110-BROWSER/ESCAPE`
		later.Go("use", func(ctx context.Context) error { // "capture by a nested Go callback": allowed
			_, err := session.Generate[gimbal.Text](ctx, prompt, gimbal.WithBrowser(b)) // want `GIMBAL110-BROWSER/ESCAPE`
			return err
		})
		return nil
	}) // browser released here
	_ = later.Wait()
	_ = kept
	return err
}

// Allowed forms.

func use(*gimbal.Browser) {}

func inScope(ctx context.Context, dir string) error {
	return gimbal.Scope(ctx, "browser-session", func(ctx context.Context) error {
		b, err := gimbal.NewBrowser(ctx, "browser", dir, dir+"/video.webm")
		if err != nil {
			return err
		}
		use(b)
		_ = b
		var alias *gimbal.Browser = b
		other := alias
		other = b
		use(other)
		_, err = session.Generate[gimbal.Text](ctx, prompt, gimbal.WithBrowser(b))
		return err
	})
}

func returnedTuple(ctx context.Context, dir string) (*gimbal.Browser, error) {
	return openBrowser(ctx, dir)
}

func namedResult(ctx context.Context, dir string) (b *gimbal.Browser, err error) {
	b, err = gimbal.NewBrowser(ctx, "browser", dir, "")
	return
}

func nestedScopes(ctx context.Context, dir string, items []string) error {
	b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
	if err != nil {
		return err
	}
	var kept *gimbal.Browser
	if err := gimbal.Scope(ctx, "child", func(ctx context.Context) error {
		use(b)
		inner := b
		kept = inner // allowed: inner is an alias of the outer browser
		return nil
	}); err != nil {
		return err
	}
	for ctx, item := range gimbal.Iterate(ctx, "items", items) {
		_ = item
		_, _ = session.Generate[gimbal.Text](ctx, prompt, gimbal.WithBrowser(b))
	}
	users := gimbal.Group(ctx, "users")
	users.Go("user", func(ctx context.Context) error {
		_, err := session.Generate[gimbal.Text](ctx, prompt, gimbal.WithBrowser(b))
		return err
	})
	gimbal.Group(ctx, "direct").Go("user", func(ctx context.Context) error {
		use(kept)
		return nil
	})
	if err := gimbal.Scope(ctx, "child", func(ctx context.Context) error {
		nested := gimbal.Group(ctx, "nested")
		nested.Go("user", func(ctx context.Context) error {
			use(b)
			return nil
		})
		return nested.Wait()
	}); err != nil {
		return err
	}
	return users.Wait()
}

func browsersFromCaller(bs []*gimbal.Browser) {
	for _, b := range bs {
		use(b)
	}
}

// Rule 1: assignment outside the owner body.

var escaped *gimbal.Browser // want `GIMBAL110-BROWSER/ESCAPE`

func assignments(ctx context.Context, dir string, items []string) (result *gimbal.Browser, err error) {
	var outer *gimbal.Browser
	err = gimbal.Scope(ctx, "owner", func(ctx context.Context) error {
		b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
		if err != nil {
			return err
		}
		result = b                                              // want `GIMBAL110-BROWSER/ESCAPE`
		escaped = b                                             // want `GIMBAL110-BROWSER/ESCAPE`
		outer, err = gimbal.NewBrowser(ctx, "browser", dir, "") // want `GIMBAL110-BROWSER/ESCAPE`
		return err
	})
	for ctx, item := range gimbal.Iterate(ctx, "items", items) {
		_ = item
		b, _ := gimbal.NewBrowser(ctx, "browser", dir, "")
		outer = b // want `GIMBAL110-BROWSER/ESCAPE`
	}
	use(outer)
	return result, err
}

func returnFromRange(ctx context.Context, dir string, items []string) *gimbal.Browser {
	for ctx, item := range gimbal.Iterate(ctx, "items", items) {
		_ = item
		b, _ := gimbal.NewBrowser(ctx, "browser", dir, "")
		return b // want `GIMBAL110-BROWSER/ESCAPE`
	}
	return nil
}

// Rule 2: storage.

type holder struct {
	b *gimbal.Browser // want `GIMBAL110-BROWSER/ESCAPE`
}

func storage(b *gimbal.Browser, h *holder, bs []*gimbal.Browser, m map[string]*gimbal.Browser, seen map[*gimbal.Browser]bool, p **gimbal.Browser, ch chan *gimbal.Browser) {
	h.b = b                  // want `GIMBAL110-BROWSER/ESCAPE`
	bs[0] = b                // want `GIMBAL110-BROWSER/ESCAPE`
	m["browser"] = b         // want `GIMBAL110-BROWSER/ESCAPE`
	seen[b] = true           // want `GIMBAL110-BROWSER/ESCAPE`
	*p = b                   // want `GIMBAL110-BROWSER/ESCAPE`
	ch <- b                  // want `GIMBAL110-BROWSER/ESCAPE`
	bs = append(bs, b)       // want `GIMBAL110-BROWSER/ESCAPE`
	_ = []*gimbal.Browser{b} // want `GIMBAL110-BROWSER/ESCAPE`
	_ = holder{b: b}         // want `GIMBAL110-BROWSER/ESCAPE`
	use(bs[0])
}

// Rule 3: interface conversion.

func keep[T any](T) {}

func identity[T any](v T) T { return v }

func toAny(b *gimbal.Browser) any {
	var v any = b  // want `GIMBAL110-BROWSER/ESCAPE`
	v = b          // want `GIMBAL110-BROWSER/ESCAPE`
	fmt.Println(b) // want `GIMBAL110-BROWSER/ESCAPE`
	_ = []any{b}   // want `GIMBAL110-BROWSER/ESCAPE`
	_ = any(b)     // want `GIMBAL110-BROWSER/ESCAPE`
	keep[any](b)   // want `GIMBAL110-BROWSER/ESCAPE`
	_ = v
	return b // want `GIMBAL110-BROWSER/ESCAPE`
}

// A generic helper instantiated with the browser keeps its type; its result
// belongs to the body containing the call.
func generic(ctx context.Context, b *gimbal.Browser) error {
	keep(b)
	use(identity(b))
	alias := identity(b)
	use(alias)
	var kept *gimbal.Browser
	err := gimbal.Scope(ctx, "child", func(ctx context.Context) error {
		kept = identity(b) // want `GIMBAL110-BROWSER/ESCAPE`
		return nil
	})
	_ = kept
	return err
}

func switched(v any) {
	switch b := v.(type) {
	case *gimbal.Browser:
		run(func() {
			use(b) // want `GIMBAL110-BROWSER/ESCAPE`
		})
	}
}

// Rule 4: captures by other function literals.

func run(func()) {}

func captures(ctx context.Context, b *gimbal.Browser) func() {
	defer func() {
		use(b) // want `GIMBAL110-BROWSER/ESCAPE`
	}()
	run(func() {
		use(b) // want `GIMBAL110-BROWSER/ESCAPE`
	})
	gimbal.Group(ctx, "other").Go("user", func(ctx context.Context) error {
		return gimbal.Scope(ctx, "inner", func(ctx context.Context) error {
			use(b) // allowed: both callbacks are scopes nested in b's owner
			return nil
		})
	})
	return func() {
		use(b) // want `GIMBAL110-BROWSER/ESCAPE`
	}
}

// Rule 5: a raw goroutine.

func goroutines(b *gimbal.Browser) {
	go use(b) // want `GIMBAL110-BROWSER/ESCAPE`
	go func() {
		use(b) // want `GIMBAL110-BROWSER/ESCAPE`
	}()
}

// Options that retain a browser.

func optionEscapes(ctx context.Context, dir string) error {
	var opts []gimbal.AgentOption
	var opt gimbal.AgentOption
	var sup gimbal.AgentOption
	outerOpts := make([]gimbal.AgentOption, 1)
	err := gimbal.Scope(ctx, "owner", func(ctx context.Context) error {
		b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
		if err != nil {
			return err
		}
		opts = append(opts, gimbal.WithBrowser(b))                                           // want `GIMBAL110-BROWSER/ESCAPE`
		opt = gimbal.WithBrowser(b)                                                          // want `GIMBAL110-BROWSER/ESCAPE`
		opt = gimbal.WithSupervisor(reviewer, "watch", gimbal.WithBrowser(b))                // want `GIMBAL110-BROWSER/ESCAPE`
		opts = append(opts, gimbal.WithSupervisor(reviewer, "watch", gimbal.WithBrowser(b))) // want `GIMBAL110-BROWSER/ESCAPE`
		local := []gimbal.AgentOption{gimbal.WithBrowser(b)}
		sup = gimbal.WithSupervisor(reviewer, "watch", local...)                                                       // want `GIMBAL110-BROWSER/ESCAPE`
		sup = gimbal.WithSupervisor(reviewer, "watch", gimbal.WithSupervisor(reviewer, "look", gimbal.WithBrowser(b))) // want `GIMBAL110-BROWSER/ESCAPE`
		inner := gimbal.WithBrowser(b)
		alias := inner
		opt = alias                                  // want `GIMBAL110-BROWSER/ESCAPE`
		outerOpts[0] = alias                         // want `GIMBAL110-BROWSER/ESCAPE`
		_ = any(alias)                               // want `GIMBAL110-BROWSER/ESCAPE`
		_ = struct{ o gimbal.AgentOption }{o: alias} // want `GIMBAL110-BROWSER/ESCAPE`
		defer func() {
			_, _ = session.Generate[gimbal.Text](ctx, prompt, local...) // want `GIMBAL110-BROWSER/ESCAPE`
		}()
		go func() {
			_, _ = session.Generate[gimbal.Text](ctx, prompt, alias) // want `GIMBAL110-BROWSER/ESCAPE`
		}()
		return nil
	})
	_, _ = session.Generate[gimbal.Text](ctx, prompt, append(opts, opt, sup, outerOpts[0])...)
	return err
}

func optionFromRange(ctx context.Context, dir string, items []string) gimbal.AgentOption {
	for ctx, item := range gimbal.Iterate(ctx, "items", items) {
		_ = item
		b, _ := gimbal.NewBrowser(ctx, "browser", dir, "")
		return gimbal.WithBrowser(b) // want `GIMBAL110-BROWSER/ESCAPE`
	}
	return nil
}

func optionSend(b *gimbal.Browser, ch chan gimbal.AgentOption) {
	ch <- gimbal.WithBrowser(b) // want `GIMBAL110-BROWSER/ESCAPE`
}

// Safe local option assembly.

func optionAssembly(ctx context.Context, dir string) error {
	b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
	if err != nil {
		return err
	}
	var outer []gimbal.AgentOption
	outer = append(outer, gimbal.WithBrowser(b))
	if err := gimbal.Scope(ctx, "child", func(ctx context.Context) error {
		outer = append(outer, gimbal.WithSupervisor(reviewer, "watch", gimbal.WithBrowser(b))) // allowed: b is the outer body's
		_, err := session.Generate[gimbal.Text](ctx, prompt, outer...)
		return err
	}); err != nil {
		return err
	}
	return gimbal.Scope(ctx, "owner", func(ctx context.Context) error {
		b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
		if err != nil {
			return err
		}
		opts := []gimbal.AgentOption{gimbal.WithBrowser(b)}
		opts = append(opts, gimbal.WithSupervisor(reviewer, "watch", gimbal.WithBrowser(b)))
		opts[0] = gimbal.WithBrowser(b)
		sup := gimbal.WithSupervisor(reviewer, "watch", opts...)
		opts = append(opts, sup)
		g := gimbal.Group(ctx, "turns")
		g.Go("turn", func(ctx context.Context) error {
			_, err := session.Generate[gimbal.Text](ctx, prompt, opts...)
			return err
		})
		if err := gimbal.Scope(ctx, "inner", func(ctx context.Context) error {
			_, err := session.Generate[gimbal.Text](ctx, prompt, sup)
			return err
		}); err != nil {
			return err
		}
		_, err = session.Generate[gimbal.Text](ctx, prompt, opts...)
		return g.Wait()
	})
}

// Options no browser reaches are not tracked, so ordinary option plumbing
// is never reported.

func plainOptions(ctx context.Context, opts ...gimbal.AgentOption) {
	var saved []gimbal.AgentOption
	run(func() {
		saved = append(saved, opts...)
		_, _ = session.Generate[gimbal.Text](ctx, prompt, saved...)
	})
	go func() {
		_, _ = session.Generate[gimbal.Text](ctx, prompt, opts...)
	}()
}

// Outside the static guarantee: an option a helper builds is opaque, so its
// outward assignment is left to the runtime owner check.

func browserOption(b *gimbal.Browser) gimbal.AgentOption {
	return gimbal.WithBrowser(b)
}

func opaqueHelper(ctx context.Context, dir string) error {
	var opt gimbal.AgentOption
	err := gimbal.Scope(ctx, "owner", func(ctx context.Context) error {
		b, err := gimbal.NewBrowser(ctx, "browser", dir, "")
		opt = browserOption(b)
		return err
	})
	_ = opt
	return err
}
