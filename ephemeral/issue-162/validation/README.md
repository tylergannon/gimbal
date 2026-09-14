# Independent validation of issue 162

The parent agent reviewed GPT Sol's implementation and independently exercised
the built `bin/gimble`. The implemented, documented first-pass behavior is
demonstrated. Implementation commit: `218799b493c7261428ab5183d2ec0abe39732e3a`.
Sol's reproducible proof and results are committed in `f13e782`.

## Observed behavior

All 17 independent checks in [results.json](results.json) passed. The probe
[verify.py](verify.py) runs real deterministic workflows, then invokes the
actual binary through both `gimble lint` and `go vet -vettool`.

- Direct map dispatch and lookup aliases in helpers run successfully without
  lint and produce GIMBLE101 with nonzero lint exits.
- Explicit switches, direct helpers, a single-target alias, and joined Group
  callbacks run and lint successfully.
- Named workflow callbacks and directly reached helpers without context
  parameters are analyzed.
- Real generic Set/SetJSON calls report repeated keys, including normal loops
  and captured outer task contexts. Mutually exclusive writes and fresh task
  contexts pass.
- Ordinary CLI help and run-prompt routing remain correct when arguments end
  in `.cfg`, including arguments naming actual vet-shaped configuration files.

Reproduce from the repository root after building:

```sh
python3 ephemeral/issue-162/validation/verify.py ./bin/gimble
go run ./ephemeral/issue-162/validation/testdata/extract ephemeral/issue-162/proof/testdata/dispatch_pass/main.go
```

The source-only extraction probe reports both `implement(ctx)` and
`review(ctx)` under their case labels, without running the workflow or
predicting `kind`. Its output is [switch-alternatives.txt](switch-alternatives.txt).
This is an acceptance probe, not the graph-extraction feature of issue 201.

## Review corrections

Independent probes exposed missed named callbacks, generic SSA duplicate
writes, ordinary CLI arguments stolen by vet routing, and a helper-domain
restriction based on the helper's own parameter types. Sol corrected these;
the final probes cover them. Earlier failing output is retained separately in
`first-results.json`, `first-generic-duplicate.json`, and `helper-before-fix.json`.

## Sol's proof inspected

The command, stdout, stderr, and exit files under `../proof/results/` show:

- All seven accepted diagnostic IDs on compiling real-API negative source,
  with clean counterpart source.
- A duplicate `role` inserted into an isolated copy of the builtin sprint
  reported before repository tests; the tracked sprint was left unchanged.
- The builtin sprint and Codex review source retain constant-key failures.
  Root production source, including loop.go, and Claude review source pass.
- `just build`, `go test -count=1 ./...`, and stock `go vet ./...` exit zero.
- `just vet` exits 3: three existing dynamic keys (sprint, group-in-task
  attestation, Codex review) and two intentional duplicate-write runtime tests
  are reported. No exemptions or source rewrites conceal them.

Thus the analyzer's acceptance proof passes; the repository-wide combined vet
gate is not green. Cleaning those source cases or deciding how intentional
misuse tests belong in the gate is separate work.

## Binary size and limits

Sol rebuilt clean baseline and implementation worktrees with the same
`go build -o OUTPUT ./cmd` flags, Go 1.27.1, and the same placeholder frontend
assets: 26,863,650 to 32,403,826 bytes, an increase of 5,540,176 bytes (20.62%).
After `just build`, the final local binary includes the built frontend and is
33,164,322 bytes. Its hash, build information, implementation head, and empty
production-source diff are recorded in [metadata.json](metadata.json).

The dispatch pass recognizes context-bearing worker callables and follows
source-visible package-local helpers. It is not whole-program proof that all
indirect dispatch is absent. No automatic fixes are supplied. Runtime misuse
checks remain the backstop.
