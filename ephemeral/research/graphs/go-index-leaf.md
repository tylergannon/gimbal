# Go research leaf index

All references below were downloaded into this flat directory. The line
routes are local files to make review reproducible without relying on a live
web page.

| Topic | Primary URL | Local artifact and line route |
| --- | --- | --- |
| Latest Go release and 1.27.1 date | https://go.dev/doc/devel/release | `go-source-go-release-history.html:473-491`; `go-local-go-version.txt:1` |
| Go 1.27 generic methods, stdversion | https://go.dev/doc/go1.27 | `go-source-go1.27-release-notes.html:450-500` |
| Go 1.26 analysis/vet/fix convergence | https://go.dev/doc/go1.26 | `go-source-go1.26-release-notes.html:502-518` |
| Go 1.23 range-over-func and stdversion | https://go.dev/doc/go1.23 | `go-source-go1.23-release-notes.html:461-471,519-523,711` |
| Go analysis framework | https://pkg.go.dev/golang.org/x/tools/go/analysis@v0.50.0 | `go-local-x-tools-analysis-doc.txt:20-155,185-280`; HTML version `go-source-x-tools-analysis-v0.50.0.html:568-598` |
| Package loading | https://pkg.go.dev/golang.org/x/tools/go/packages@v0.50.0 | `go-local-x-tools-packages-doc.txt:1-116`; HTML version `go-source-x-tools-packages-v0.50.0.html:568-598,1078-1100` |
| go/types identity and generics | https://pkg.go.dev/go/types | `go-local-go-types-doc.txt:693-820,1528-1610,1748-1802` |
| Public AST cursor navigation | https://pkg.go.dev/golang.org/x/tools/go/ast/inspector@v0.50.0 | `go-local-x-tools-inspector-all.txt:55-238`; HTML version `go-source-x-tools-inspector-v0.50.0.html:572-602,866-923` |
| Syntactic CFG | https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/ctrlflow@v0.50.0 | `go-local-x-tools-ctrlflow-all.txt:1-46`; `go-source-x-tools-ctrlflow-v0.50.0.html:946-1065` |
| SSA analyzer | https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildssa@v0.50.0 | `go-local-x-tools-buildssa-all.txt:1-29`; `go-source-x-tools-buildssa-v0.50.0.html:946-1010` |
| SSA CFG, calls, generics, range yield | https://pkg.go.dev/golang.org/x/tools/go/ssa@v0.50.0 | `go-local-x-tools-ssa-all.txt:199-240,384-480,895-1075,1139-1160`; HTML `go-source-x-tools-ssa-v0.50.0.html:5677-5688,5958,6565-6576,8439-8451` |
| Generic/static callee resolution | https://pkg.go.dev/golang.org/x/tools/go/types/typeutil@v0.50.0 | `go-local-x-tools-typeutil-all.txt:1-47`; HTML `go-source-x-tools-typeutil-v0.50.0.html:572-602,1223-1238` |
| Callgraph contract | https://pkg.go.dev/golang.org/x/tools/go/callgraph@v0.50.0 | `go-local-x-tools-callgraph-all.txt:1-103`; HTML `go-source-x-tools-callgraph-v0.50.0.html:567-590` |
| CHA/RTA/static/VTA algorithms | https://pkg.go.dev/golang.org/x/tools/go/callgraph | `go-local-x-tools-callgraph-{cha,rta,static,vta}-all.txt:1-60`; downloaded pages `go-source-x-tools-callgraph-{cha,rta,static,vta}-v0.50.0.html` |
| Internal typeindex status | https://pkg.go.dev/golang.org/x/tools/internal/typesinternal/typeindex@v0.50.0 | `go-source-x-tools-internal-typeindex-v0.50.0.html:977-1028,1245-1247`; module directory listing `go-source-x-tools-module-latest.html:2799-2806,3473-3484` |
| go generate behavior | https://pkg.go.dev/cmd/go#hdr_Generate_Go_files_by_processing_source | `go-local-go-help-generate.txt:1-55`; downloaded page `go-source-cmd-go-generate.html` |
| go vet and -vettool | https://pkg.go.dev/cmd/go#hdr_Testing_and_analysis | `go-local-go-help-vet.txt:1-32`; `go-local-cmd-vet-doc.txt:1-75`; `go-local-go-help-tool.txt:1-22` |

## Gimble-local routes

- Public API snapshot: `go-public-api-go-doc.txt:1-260`.
- Scope/context/set behavior: `go-local-scope-lines.txt:45-49,76-89,127-181`.
- Session/generic Generate/ownership: `go-local-session-lines.txt:14-74`.
- Group callback/join: `go-local-group-lines.txt:19-98`.
- Loop task callback and per-task scope: `go-local-loop-lines.txt:127-164`.
- Current versions and tool block: `go-local-gomod-lines.txt:1-48`.
- Existing requested checks and known first-pass shape: `go-local-issue-162-lines.txt:1`.
- Executable feasibility probe and exact output: `go-feasibility-probe.go:1-230`, `go-feasibility-probe-output.txt:1-14`.
