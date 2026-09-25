package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestClosedScopeValuesDoNotReachParentOrSiblingPrompts(t *testing.T) {
	for _, templated := range []bool{false, true} {
		name := "default"
		if templated {
			name = "template"
		}
		for _, failed := range []bool{false, true} {
			exit := "return"
			if failed {
				exit = "error"
			}
			t.Run(name+"/"+exit, func(t *testing.T) {
				var prompts []string
				adapter := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
					prompts = append(prompts, prompt)
					return "done", nil
				}}
				var opts []AgentOption
				if templated {
					opts = []AgentOption{WithScopeTemplate(`{{range .Values}}{{.Key}}: {{.Value}}
{{end}}`)}
				}
				childErr := errors.New("child failed")
				err := runTest(t, bind(adapter, "m", "worker"), func(ctx context.Context) error {
					Set(ctx, "language", "parent-language")
					SetJSON(ctx, "review", review{Objections: []string{"parent-review"}})
					worker := NewSession(ctx, "worker", t.TempDir())
					if _, err := worker.Generate[Text](ctx, "Inspect the context.", opts...); err != nil {
						return err
					}
					err := Scope(ctx, "child", func(child context.Context) error {
						Set(child, "language", "child-language")
						SetJSON(child, "review", review{Objections: []string{"child-review"}})
						Set(child, "child-only", "child-only-value")
						SetJSON(child, "child-json", review{Objections: []string{"child-json-value"}})
						if _, err := worker.Generate[Text](child, "Inspect the context.", opts...); err != nil {
							return err
						}
						if failed {
							return childErr
						}
						return nil
					})
					if failed && !errors.Is(err, childErr) || !failed && err != nil {
						t.Fatalf("child returned %v (failed=%v)", err, failed)
					}
					data := scopeData(ctx)
					if len(data.Values) != 2 || len(data.By) != 2 {
						t.Fatalf("parent retained child entries: %+v", data)
					}
					if _, err := worker.Generate[Text](ctx, "Inspect the context.", opts...); err != nil {
						return err
					}
					return Scope(ctx, "sibling", func(sibling context.Context) error {
						_, err := worker.Generate[Text](sibling, "Inspect the context.", opts...)
						return err
					})
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(prompts) != 4 {
					t.Fatalf("got %d prompts, want 4", len(prompts))
				}
				for _, marker := range []string{"parent-language", "parent-review"} {
					if !strings.Contains(prompts[0], marker) || strings.Contains(prompts[1], marker) {
						t.Errorf("parent value %q missing or not shadowed: %q", marker, prompts[:2])
					}
				}
				for _, marker := range []string{"child-language", "child-review", "child-only-value", "child-json-value"} {
					if !strings.Contains(prompts[1], marker) || strings.Contains(prompts[0], marker) {
						t.Errorf("child value %q not confined to child: %q", marker, prompts[:2])
					}
				}
				if !slices.Equal(prompts[2:], []string{prompts[0], prompts[0]}) {
					t.Errorf("parent and sibling prompts did not restore the original context: %q", prompts)
				}
			})
		}
	}
}
