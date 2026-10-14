# Adversarial review: release diagnosis and remediation plan, round 03

## Target

`ephemeral/research/release-diagnosis/diagnosis.md`, now including the
`## Remediation plan, including removal of Python` section. Judged against the
request recorded in it (plus the added instruction to remove Python, with the
replacement language delegated), `AGENTS.md` ("As simple as possible", "Build
what was asked", "Proof is what you saw", the hook budget), and the source.

The launch prompt supplied only target, artifact path, and read-only limits;
nothing narrowed scope.

## Evidence inspected

- Diagnosis sections after round 02 (independent-review account, plan).
- Python inventory (`git ls-files '*.py'`, non-ephemeral: 24 files):
  - `scripts/`: six entry points (`ci_scope.py`, `prepare-release.py`,
    `dispatch-release.py`, `release-assets.py`, `release_timing.py`,
    `pre-commit.py`) and six `*_test.py` files (`auto_release_test`,
    `ci_scope_test`, `fast_hook_test`, `release_assets_test`,
    `release_timing_test`, `staged_hook_test`). The plan's "six entry points /
    six test files" is correct.
  - `.agents/skills/*/scripts/*.py`: 12 more Python files
    (`resolve_models.py` x7, `chapter.py`, `promise.py`, `run_evals.py`,
    `semantic_index_metrics.py`, `ledger.py`).
- Python call sites: `.github/workflows/{auto-release,release,release-performance}.yml`
  (including `python3 -B -m unittest discover` at `auto-release.yml:62`),
  `Justfile:60-65`, `lefthook.yml:12`, `scripts/generate-appcast.sh:32`
  (inline), `scripts/release-macos.sh:146` (preflight `for tool in go python3`)
  and `:305-316` (three inline `json` one-liners), `docs/context-budget.md:18`.
- `scripts/pre-commit.py` header: 2.7 s budget "leaving 300 ms for Python and
  lefthook startup", test-input globs include `scripts/*.py`, `.py` in
  `CODE_SUFFIXES`.
- `ensure_issue`, `main` in `release_timing.py` (as in earlier rounds).

## Findings

No material defects in the diagnosis. The plan is coherent, honors "no
wrapper/no shim/no Python fallback", keeps shell for build/signing, and its
proof section requires real runs rather than committed proof artifacts.
Findings concern plan completeness.

### 1. issue: "Python removal" scope is ambiguous against 12 skill scripts

The plan enumerates six scripts plus two inline sites and says "delete the old
Python files and runtime requirements", but `.agents/skills/` holds 12 further
tracked `.py` files (`resolve_models.py` copies, `promise.py`, `ledger.py`,
`run_evals.py`, ...). Its proof item 2 says "maintained automation has no
executable Python references or .py files" — read literally that fails while
those remain; read narrowly it silently exempts them. Tyler's instruction was
to remove Python. The plan should state explicitly whether `.agents/skills/`
is in scope (it appears to be vendored/borrowed content per the
`update-borrowed-skills` skill, so removal there may be wrong or unwanted), and
ask rather than assume.

### 2. issue: incomplete inventory of Python dependencies in the plan

Named: appcast verification and notary JSON parsing. Not named, but present:
the `python3` tool-presence preflight at `release-macos.sh:146`, which will
break the Mac release after Python is removed from the runner if left; the
`docs/context-budget.md` snippet; `.gitignore`/`scripts/__pycache__`; the
`scripts/*.py` glob and `.py` suffix inside the replacement hook's own input
list; and the `unittest discover` step in `auto-release.yml:62`, which is a
test gate and must be replaced by `go test` of the new package or the gate
silently disappears. Add these to the plan's "update workflows... references"
bullet, otherwise the "no executable Python references" proof will find them
late.

### 3. issue: plan does not say where the Go replacement lives or how the hook gets a binary

The hook's three-second budget currently pays Python startup (300 ms). A Go
replacement must be a prebuilt binary or `go run` (compile cost at every
commit). The plan says "compilation... must not consume the timed hook path;
explicit setup/warmup handles preparation" but not what lefthook invokes on a
fresh clone with no warmup. Without that the first commit exceeds budget and
prints the slow-check message on every new worktree. Also unstated: package
location (`internal/...` vs `cmd/...`) and that it adds no exported API name,
as `AGENTS.md` requires. A one-line decision on each avoids reopening this
mid-implementation.

### 4. nitpick: behavior additions bundled with the language port

Bounded retry for transient API errors and "hook check runs independently of
scan failure" are fixes for diagnosed defects, but they are behavior changes
mixed with a mechanical port. Fine, but implementing them as separate commits
keeps the port reviewable against the Python behavior ("preserve exact-revision
checks, safe archive restore, atomic tags").

### 5. nitpick: pending-review sentence

The final line says the plan is "pending a further Claude review"; replace
with this round's outcome after adjudication.

## Outcome

material findings remain
