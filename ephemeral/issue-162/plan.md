# Issue 162 implementation plan

Two research agents, both using gpt-5.6-luna, checked current official Go/x/tools documentation and the installed Go 1.27.1 / x/tools v0.50.0 source. Use Go's standard analysis framework: one analyzer executable can support standalone execution and the vet protocol.

1. Add one internal `go/analysis` analyzer and a `cmd/gimblelint` executable using `singlechecker`. Register it in the existing `go.mod` tool block, giving authors `go tool gimblelint ./...`. Have `just vet` run ordinary `go vet ./...`, then the custom analyzer; also verify `go vet -vettool=./bin/gimblelint ./...` against a built executable. Custom vet tools replace the default checker, so both invocations are needed to retain standard checks. Merely adding a tool directive does not make plain `go build` run it.

2. Implement the seven accepted checks: constant Set/SetJSON keys; duplicate writes, including repeated writes through an outer loop context; wrong child context; writes in raw goroutines; reserved task keys; Background/TODO contexts; and dynamic worker dispatch. Use Go type information for symbol identity and constants, syntax for callback boundaries, and SSA identity/dominance for duplicate writes. Treat Tasks iteration contexts as fresh. Limit worker-dispatch analysis to workflow entrypoints, recognized callbacks, and directly reachable helpers; catch collection calls and simple local aliases while allowing direct helpers, switches, and single-target aliases. Document detection limits and leave proposed follow-up rules out.

3. Prove each rule with positive and negative `analysistest` fixtures. Run deterministic examples without lint, then assert lint's nonzero exit and diagnostic IDs. Exercise both command paths, the builtin sprint, loop.go, and both specified review workflows. Preserve and demonstrate the sprint's existing formatted-key failure before any separate cleanup; do not exempt it to obtain a green gate.

4. Start with diagnostics only. Go supports `go fix -fixtool` and vet fixes through `SuggestedFixes`, but these rules generally require author decisions about keys, scope ownership, or dispatch. Keep that extension point without inventing automatic rewrites.

Sources: https://go.dev/cmd/go/ ; https://go.dev/ref/mod#go-mod-file-tool ; https://pkg.go.dev/golang.org/x/tools/go/analysis ; https://go.googlesource.com/tools/+/refs/heads/master/go/analysis/singlechecker/singlechecker.go
