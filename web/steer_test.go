package web

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/runlog"

	// src/routes, reached through the link tree skgo generates: a route
	// directory is named after the URL it serves, so that tree is its own
	// module and Go reaches into it only through these links. links.json
	// maps this one back to web/src/routes.
	routes "github.com/tylergannon/gimble/internal/skgo/links/onzggl3sn52xizlt"
)

// TestSteerFormReachesTheSessionAPersonIsWatching is the page half of #176:
// the form the run page posts steers the same run Runtime.Steer does, and
// the run log records it with Source "person". It calls the handler with the
// runtime's own context because that is what the remote transport does — a
// remote function runs on the request's context, which descends from the
// server's BaseContext, which is this.
func TestSteerFormReachesTheSessionAPersonIsWatching(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	b := &blocking{}
	const (
		session = "lap.1/coder.1"
		turn    = "lap.1/coder.1/turn.1"
		scope   = "lap.1"
	)
	var runWG sync.WaitGroup
	runWG.Go(func() {
		_ = runtime.Run(ctx, "steering", map[string]gimble.ModelBinding{"coder": {Adapter: b, Model: "m"}}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				coder := gimble.NewSession(ctx, "coder", "/w")
				_, err := coder.Generate[gimble.Text](ctx, "wait")
				return err
			})
		})
	})

	startedTurns(t, b, 1)
	id := runID(t, project)

	sent, err := routes.Skgo_steer(runtime.ctx, routes.Steer{Run: id, Session: session, Message: "  look at the tests  "})
	if err != nil || !sent.Landed {
		t.Fatalf("steer = %+v, %v; want it landed", sent, err)
	}

	// A message typed at a session with nothing running is not an error. The
	// person is told it did not land, which is the current policy: a steer
	// sent to an idle session is dropped.
	if err := runtime.KillTurn(id, turn, "tyler", "that is enough"); err != nil {
		t.Fatal(err)
	}
	runWG.Wait()

	var steered []gimble.LifecycleRecord
	if err := runlog.Read[gimble.LifecycleRecord](ctx, filepath.Join(project, "runs", id), func(record gimble.LifecycleRecord) error {
		if _, ok := record.Event.(gimble.Steer); ok {
			steered = append(steered, record)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(steered) != 1 {
		t.Fatalf("Steer records = %+v, want one", steered)
	}
	steer := steered[0].Event.(gimble.Steer)
	// The message is recorded trimmed: what the form posts carries whatever
	// whitespace the textarea held, and the agent is sent the words.
	if steered[0].Scope != scope || steered[0].Session.Value != session || steered[0].Turn.Value != turn ||
		steer.Source != "person" || steer.Target != session || !steer.Landed || steer.Message != "look at the tests" {
		t.Errorf("Steer record = %+v %+v, want Source person on %s", steered[0], steer, turn)
	}
}

// TestSteerFormRefusesWhatItCannotDeliver covers what the page can put in
// front of a person: an empty box, a run that has finished since the page
// was drawn, and a session that is not part of it.
func TestSteerFormRefusesWhatItCannotDeliver(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := NewRuntime(ctx, project, WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	b := &blocking{}
	var runWG sync.WaitGroup
	runWG.Go(func() {
		_ = runtime.Run(ctx, "steering", map[string]gimble.ModelBinding{"coder": {Adapter: b, Model: "m"}}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				coder := gimble.NewSession(ctx, "coder", "/w")
				_, err := coder.Generate[gimble.Text](ctx, "wait")
				return err
			})
		})
	})
	startedTurns(t, b, 1)
	id := runID(t, project)

	// An empty box is the field's problem, not the server's: kit hangs the
	// message off the input and the page stays as it is.
	var invalid *skgo.Invalid
	if _, err := routes.Skgo_steer(runtime.ctx, routes.Steer{Run: id, Session: "lap.1/coder.1", Message: "   "}); !errors.As(err, &invalid) {
		t.Fatalf("empty message = %v, want an issue on the field", err)
	}
	if len(invalid.Issues) != 1 || invalid.Issues[0].Field != "message" {
		t.Errorf("issues = %+v, want one on message", invalid.Issues)
	}

	var status *skgo.HTTPError
	if _, err := routes.Skgo_steer(runtime.ctx, routes.Steer{Run: id, Session: "lap.1/nobody.1", Message: "hello"}); !errors.As(err, &status) || status.Status != 404 {
		t.Errorf("unknown session = %v, want a 404", err)
	}

	if err := runtime.KillScope(id, "lap.1", "tyler", "that is enough"); err != nil {
		t.Fatal(err)
	}
	runWG.Wait()

	if _, err := routes.Skgo_steer(runtime.ctx, routes.Steer{Run: id, Session: "lap.1/coder.1", Message: "too late"}); !errors.As(err, &status) || status.Status != 404 {
		t.Errorf("finished run = %v, want a 404", err)
	}
}
