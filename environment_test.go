package gimbal

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/runstore"
)

type environmentAdapter struct {
	*fake
	created []string
}

func (a *environmentAdapter) CreateSession(ctx context.Context, model, effort, dir string) (string, error) {
	a.created = append(a.created, effects.Environment(ctx))
	return a.fake.CreateSession(ctx, model, effort, dir)
}
func TestSessionCapturesInheritedEnvironment(t *testing.T) {
	var turns, closes []string
	a := &environmentAdapter{fake: &fake{answer: func(ctx context.Context, _, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turns = append(turns, effects.Environment(ctx))
		return "ok", nil
	}, onClose: func(ctx context.Context, _ string) { closes = append(closes, effects.Environment(ctx)) }}}
	err := Run(Project(t.Context(), t.TempDir()), string(runstore.NewRunID()), "environment", bind(a, "test", "agent"), func(ctx context.Context) error {
		return Scope(WithEnvironment(ctx, "local"), "child", func(child context.Context) error {
			s := NewSession(child, "agent", ".")
			moved := WithEnvironment(child, "other")
			_, err := s.Generate[Text](moved, "hello")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.created) != 1 || a.created[0] != "local" || len(turns) != 1 || turns[0] != "local" || len(closes) != 1 || closes[0] != "local" {
		t.Fatalf("creation %v turns %v closes %v", a.created, turns, closes)
	}
}

func TestExplicitUnknownEnvironmentDoesNotRunLocally(t *testing.T) {
	err := Run(Project(t.Context(), t.TempDir()), string(runstore.NewRunID()), "environment", nil, func(ctx context.Context) error {
		_, _, _, err := RunCommand(WithEnvironment(ctx, "unknown"), "command", ".", "sh", "-c", "exit 0")
		if err == nil {
			t.Fatal("unknown environment ran locally")
		}
		if err := Service(WithEnvironment(ctx, "unknown"), "service", ".", "true"); err == nil {
			t.Fatal("unknown environment ran a local service")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
