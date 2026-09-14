# Add stable model aliases

URL: https://github.com/tylergannon/gimble/issues/11
State: closed
Milestone: None
Updated: 2026-09-13T16:46:48Z

## Problem

Pipeline definitions currently embed provider-native model IDs or depend on entry-point defaults. That makes graphs noisy, turns every graph into its own model-version policy, and makes a coordinated model upgrade require edits across authored workflows.

The archived Attractor implementation isolated this concern in a small alias resolver while continuing to accept raw model IDs.

## Request

Add a repo-owned model alias registry used consistently by graph execution, the CLI, and MCP-started runs.

Raw provider-native IDs must remain a supported escape hatch. Alias resolution should return both provider and concrete model, reject an explicitly conflicting provider, and fail clearly for a known alias that is temporarily unsupported.

## Acceptance criteria

- A small documented set of aliases resolves to provider plus concrete model.
- Graph, CLI, and MCP execution use the same resolver.
- Raw model IDs continue to work without registration.
- An explicit provider that conflicts with an alias is rejected before harness activity.
- Unknown aliases are not silently rewritten.
- Tests cover resolution, raw IDs, conflicts, unsupported aliases, and entry-point consistency.
- User-facing documentation explains that aliases are maintained policy and raw IDs are the stability escape hatch.

## Source reference

Archived Attractor resolver at 0aca8b7:
https://github.com/tylergannon/attractor/blob/0aca8b748e6ecc23446fc690d2b66690b77fe0d3/internal/modelalias/modelalias.go

