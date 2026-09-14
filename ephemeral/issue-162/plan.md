# Issue 162 implementation plan

The distribution decision is one existing Gimble binary. The internal
`go/analysis` analyzer is linked into `cmd`; `gimble lint ./...` enters
`singlechecker`, while the same process recognizes Go's `-V=full`, `-flags`,
and unit-config protocol calls before ordinary CLI parsing. No second command
or module tool is added.

1. Implement the seven accepted checks with type information, syntax, and SSA:
   constant Set/SetJSON keys; duplicate writes including outer contexts reused
   by loops; wrong child contexts; writes in raw goroutines; reserved task
   keys; Background/TODO contexts; and dynamic worker dispatch. Keep dynamic
   dispatch bounded to workflow callbacks and their directly reached,
   source-visible package helpers. Do not add suggested fixes or proposed
   follow-up rules.

2. Make `just vet` run ordinary Go vet, build the normal Gimble binary, and run
   its lint command. Prove direct `go vet -vettool=/absolute/path/to/gimble` as
   a separate protocol invocation because it replaces ordinary vet analyzers.
   Preserve normal server and `run-prompt` parsing when ordinary arguments end
   in `.cfg`.

3. Cover every rule and non-report with generic Set/SetJSON `analysistest`
   fixtures. Exercise deterministic runnable map and alias dispatch failures,
   plus explicit switch, direct helper, Group callback, range task context, and
   single-target alias passes through the built CLI.

4. Record exact command streams and exit codes for the current sprint's
   required constant-key failure, root `loop.go`, both review workflows, an
   injected same-scope duplicate role in an isolated copy, stock tests/vet,
   and same-flags before/after binary sizes. The required sprint failure stays
   in source and keeps the combined vet gate nonzero.
