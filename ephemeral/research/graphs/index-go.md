# Route: Go capabilities and probe

Use this route for package loading, typed API discovery, AST/CFG/SSA, call graphs, toolchain versions, or the local executable probe. The Go leaf/report are feasibility evidence, not proof of a complete analyzer or graph generator.

- Current Go/x/tools versions and corpus provenance: `go-report.md:7-21`; leaf map `go-index-leaf.md:5-21`.
- Minimal loading/analysis stack: `go-report.md:42-70`.
- Exact probe command/output and what it demonstrates: `go-report.md:72-111`; `go-feasibility-probe-output.txt:1-12`.
- Useful first-pass graph facts: `go-report.md:113-133`.
- Set/session scoping limits and counterexamples: `go-report.md:135-206`.
- Callgraph approximation and algorithm choices: `go-report.md:208-234`.
- Go release changes relevant to generic Generate and range Tasks: `go-report.md:236-252`.
- `go generate`/`go vet` integration boundary: `go-report.md:254-278`.
- Direct local API bookmarks by topic: `go-index-leaf.md:25-38`.

Adjacent routes: [index-rules.md](index-rules.md) for the Gimble semantic contract; [index-runtime.md](index-runtime.md) for what static analysis cannot observe at runtime.
