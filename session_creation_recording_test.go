package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tylergannon/gimbal/runstore/directory"
	"path/filepath"
	"testing"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/runstore"
)

type admissionRecordingAdapter struct {
	*fake
	failure         error
	nativeOnFailure string
	attempts        []backend.ExecutionIdentity
}

func (a *admissionRecordingAdapter) CreateSession(ctx context.Context, model, effort, dir string) (string, error) {
	a.attempts = append(a.attempts, effects.ExecutionIdentityFromContext(ctx))
	if len(a.attempts) == 1 {
		return a.nativeOnFailure, a.failure
	}
	return a.fake.CreateSession(ctx, model, effort, dir)
}

func TestSessionAdmissionRetryHasDistinctRecordedAttempts(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "uncertain"}[uncertain], func(t *testing.T) {
			store := directory.Store{Root: filepath.Join(t.TempDir(), "runs")}
			var failure error = &backend.ContractRejected{Message: "no work admitted"}
			if uncertain {
				failure = &backend.DeliveryUncertain{RequestID: "lost", Err: errors.New("connection lost")}
			}
			a := &admissionRecordingAdapter{fake: &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
				return "ok", nil
			}}, failure: failure}
			err := Run(WithRunStore(Project(t.Context(), t.TempDir()), store), string(runstore.NewRunID()), "retry", bind(a, "test", "agent"), func(ctx context.Context) error {
				s := NewSession(ctx, "agent", ".")
				if len(a.attempts) != 0 {
					t.Fatal("NewSession dispatched prompt-driven admission")
				}
				if _, err := s.Generate[Text](ctx, "work"); !errors.Is(err, failure) {
					t.Fatalf("first failure: %v", err)
				}
				_, err := s.Generate[Text](ctx, "work")
				if uncertain {
					if !errors.Is(err, failure) {
						t.Fatalf("uncertain retry: %v", err)
					}
					return nil
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			want := 2
			if uncertain {
				want = 1
			}
			if len(a.attempts) != want {
				t.Fatalf("admissions=%d want=%d", len(a.attempts), want)
			}
			if !uncertain && (a.attempts[0].ActionID == a.attempts[1].ActionID || a.attempts[0].SessionID != a.attempts[1].SessionID || a.attempts[0].ActionID == a.attempts[0].SessionID) {
				t.Fatalf("creation identity collision: %+v", a.attempts)
			}
		})
	}
}

func TestUncertainCreationWithNativeIDCannotDispatchTurn(t *testing.T) {
	failure := &backend.DeliveryUncertain{RequestID: "lost", Err: errors.New("lost reply")}
	turns := 0
	a := &admissionRecordingAdapter{fake: &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		turns++
		return "ok", nil
	}}, failure: failure, nativeOnFailure: "owned-native"}
	err := Run(Project(t.Context(), t.TempDir()), string(runstore.NewRunID()), "unknown-native", bind(a, "test", "agent"), func(ctx context.Context) error {
		s := NewSession(ctx, "agent", ".")
		for range 2 {
			if _, err := s.Generate[Text](ctx, "work"); !errors.Is(err, failure) {
				t.Fatalf("creation error: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if turns != 0 || len(a.attempts) != 1 {
		t.Fatalf("admissions=%d turns=%d", len(a.attempts), turns)
	}
	if len(a.closed) != 1 || a.closed[0] != "owned-native" {
		t.Fatalf("native ownership lost: %v", a.closed)
	}
}

func TestGenerateInvocationGroupsCorrectionTurns(t *testing.T) {
	store := &memoryStore{}
	calls := 0
	a := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		calls++
		if calls == 1 {
			return "", ErrInvalidOutput
		}
		return "ok", nil
	}}
	id := runstore.NewRunID()
	err := Run(WithRunStore(Project(t.Context(), t.TempDir()), store), string(id), "generation", bind(a, "test", "agent"), func(ctx context.Context) error {
		s := NewSession(ctx, "agent", ".")
		for range 2 {
			if _, err := s.Generate[Text](ctx, "work"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	generations := map[runstore.NodeID]int{}
	var turns []runstore.Node
	for _, n := range store.runs[id].nodes {
		if n.Name == "generate" {
			generations[n.ID] = 0
		}
		if n.Name == "turn" {
			turns = append(turns, n)
		}
	}
	if len(generations) != 2 || len(turns) != 3 {
		t.Fatalf("generations=%d turns=%d", len(generations), len(turns))
	}
	for _, n := range turns {
		if _, ok := generations[n.Parent]; !ok {
			t.Fatalf("turn outside Generate invocation: %+v", n)
		}
		generations[n.Parent]++
	}
	counts := map[int]int{}
	for _, n := range generations {
		counts[n]++
	}
	if counts[2] != 1 || counts[1] != 1 {
		t.Fatalf("correction grouping: %v", generations)
	}
}
