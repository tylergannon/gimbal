package gimbal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

//go:generate go tool polytype --validate

// AgentOption is an argument to Generate or PromiseLoop. The same options
// are a supervisor's own where WithSupervisor attaches it, so a supervisor
// can have supervisors of its own.
type AgentOption func(*options)

type options struct {
	supervisors   []supervisor
	scopeTemplate string
}

type supervisor struct {
	session     *Session
	instruction string
	opts        []AgentOption
}

// WithSupervisor attaches a supervisor to a turn, including a PromiseLoop
// planner's turn: a session of its own and an instruction saying what to
// watch for. Each completed provider-exposed thinking item is screened by
// Jev once per attached supervisor. A likely concern starts a supervisor
// look, whose objections are steered into the worker turn. TYPESAFE_API_KEY
// is required; there is no timed supervision. Providers without completed
// thinking events do not trigger automatic reviews.
// When the worker finishes, Generate cancels and joins its supervisors before
// returning the worker's result and error. opts can attach supervisors to the
// supervisor's own looks.
func WithSupervisor(session *Session, instruction string, opts ...AgentOption) AgentOption {
	return func(o *options) {
		o.supervisors = append(o.supervisors, supervisor{session: session, instruction: instruction, opts: opts})
	}
}

// WithScopeTemplate renders the scope for this one Generate call through the
// text/template tmpl instead of the runtime's own rendering. The template's
// argument is the scoped data that rendering would use, a ScopeData: every
// value visible from the ctx's scope, outermost scope first, and for each
// key the value of the nearest scope that set it. Its output is appended to
// the prompt in place of the default text, and a template that renders to
// nothing leaves the prompt alone.
//
// tmpl is the template's text, and like the prompt it must be readable from
// the source: a compile-time string constant, or a variable the embed
// directive fills from a file, which a long template reads better as.
// GIMBAL109 checks that. Gimbal parses each text once and keeps it; a
// template that cannot be parsed or cannot render is the error Generate
// returns, before any model is called.
//
// It shapes the scope for the Generate call it is given to. A supervisor's
// look is built from the worker's transcript and carries no scope context, so
// this does nothing among a supervisor's own options or as a PromiseLoop
// option.
func WithScopeTemplate(tmpl string) AgentOption {
	return func(o *options) { o.scopeTemplate = tmpl }
}

func apply(opts []AgentOption) options {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// review is a supervisor's answer to one look at the work.
type review struct {
	// Each objection is one thing the agent under review must change or stop doing, written as an instruction to that agent. Leave the list empty when you have no objection.
	Objections []string `json:"objections"`
}

// supervise is Generate with supervisors attached.
func supervise(ctx context.Context, s *Session, prompt string, supervisors []supervisor, started *TurnStarted, output Output) ([]byte, error) {
	history := filepath.Join(runDir(ctx), "sessions", s.id+".jsonl")
	for _, sup := range supervisors {
		s.mu.Lock()
		turn := fmt.Sprintf("%s/turn.%d", s.id, s.turns+1)
		s.mu.Unlock()
		if scope, err := current(ctx); err == nil {
			scope.run.event(scope.key, s.id, turn, SuperviseAttached{Reviewer: sup.session.id, Worker: turn, Instruction: sup.instruction})
		}
	}
	return superviseWithJev(ctx, s, prompt, supervisors, started, history, output)
}

// clipText returns an owned string no larger than limit. Owning the clipped
// bytes matters here: retaining a slice of a large tool result would retain
// the large result's backing storage too.
func clipText(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return strings.Clone(text)
	}
	keep := limit
	for {
		marker := fmt.Sprintf(" [... %d bytes omitted]", len(text)-keep)
		if len(marker) >= limit {
			return strings.Clone(marker[:limit])
		}
		newKeep := limit - len(marker)
		if newKeep == keep {
			prefix := strings.ToValidUTF8(text[:keep], "")
			return strings.Clone(prefix) + marker
		}
		keep = newKeep
	}
}

func transcriptIdentity(value string) string {
	const visibleBytes = 160
	if len(value) <= visibleBytes {
		return strings.Clone(value)
	}
	digest := sha256.Sum256([]byte(value))
	return clipText(value, visibleBytes) + fmt.Sprintf(" sha256=%x", digest)
}
