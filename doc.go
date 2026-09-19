// Package gimble runs agent workflows written as ordinary Go, independently of
// any particular agent host or coding-agent environment. A workflow can use
// Codex, Claude Code, another harness, or an adapter supplied by its author.
//
// Project places the project directory in a context. Run uses that context to
// establish the root scope and durable record for one workflow run.
//
// Workflows use normal Go control flow. Scope names a bounded segment of work;
// Group provides an observable form of errgroup-style concurrency; Iterate
// scopes the items of a finite collection; PromiseLoop lets a planner select
// tasks adaptively. Gimble supplies these runtime primitives, not named
// tactics: retries, critique rounds, bake-offs, and delivery methods remain
// visible in the workflow that needs them.
//
// NewSession creates a conversation owned by the current scope. Generate runs
// a blocking turn and returns either Text or a schema-bearing Output. Set and
// SetJSON record the run's data in the ctx's scope; Generate appends that
// scope's rendered context to the prompt itself, as prompt + "\n\n" + the
// render, or nothing when the scope holds no values. A workflow's prompt to
// Generate is therefore a compile-time constant. One call can shape that
// context its own way with WithScopeTemplate, whose template is a constant
// or an embedded file for the same reason: what the agent is sent stays
// readable in the source.
//
// RunCommand runs a command in the current scope and blocks until it exits,
// returning its exit code, stdout, and stderr. Check instead records that
// result directly in the scope context for a following agent turn to assess.
// Service starts a required foreground command through zsh and makes the
// current scope own its lifetime. The run records each of them beside the
// scope's turns.
//
// Every operation follows context.Context. Returning from a scope closes its
// sessions and stops its services, Group.Wait joins its children, and
// cancelling a run interrupts its agent work and services. See the package
// examples for complete, compiling uses of runs and groups.
package gimble
