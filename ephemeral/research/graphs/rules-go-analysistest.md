# Go analysistest reference snapshot

Source: https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest
Fetched: 2026-09-14
Purpose: fixture and proof feasibility for Gimble lints.

Relevant page excerpts (page lines as fetched 2026-09-14):

> `Run` applies an analysis to packages from `go list`, loads them with
> `golang.org/x/tools/go/packages`, and checks diagnostics and facts specified
> by `// want ...` comments. (lines 227-250)

> A diagnostic expectation is a regular expression in a `// want` comment;
> fact expectations name the object declared on that line. Unexpected or
> unmatched diagnostics and facts fail the test. (lines 235-250)

> `RunWithSuggestedFixes` additionally applies suggested fixes and verifies
> golden-file output. (lines 251-281)

Implications for Gimble: every rule needs a positive and negative fixture, and
the negative cases should include branch arms, context derivation, loop and
range-over-function bodies, aliases, and intentionally unknown interprocedural
flows. `analysistest` proves diagnostic behavior; it does not prove that a
runtime workflow's dynamic behavior obeys the rules.
