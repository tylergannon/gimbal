# Adversarial review: release diagnosis and remediation plan, round 04

## Target

The entire current `ephemeral/research/release-diagnosis/diagnosis.md`:
diagnosis, evidence sections, round-01..03 adjudications, and the revised
remediation plan (language decision, skill-helper scope, hook startup
decision, proof list). Judged against the recorded user request,
`AGENTS.md` ("As simple as possible", "Build what was asked", "Proof is what
you saw", hook rules), and the source.

The launch prompt supplied only target, artifact path, and read-only limits.
Nothing narrowed scope. (The file name's 1423 timestamp precedes round 03's
1425; the name was supplied by the caller and is kept.)

## Evidence inspected

- Whole diagnosis file (307 lines), especially the new plan text.
- Rounds 01-03 and the diagnosis's account of them. Diagnosis and publication
  facts were re-verified in rounds 01-02 and are unchanged by this revision.
- `scripts/pre-commit.py` (full), `lefthook.yml`, `.gitignore`
  (no `pycache` entry: confirmed), `go.mod` (`gopkg.in/yaml.v3 v3.0.1` is a
  direct requirement: confirmed), `internal/automation` (does not exist yet).
- `.agents/skills/*/scripts/*.py`: the seven `resolve_models.py` copies import
  `yaml`; `promise.py` uses `fcntl`; twelve helpers total, matching the plan's
  breakdown (7 + chapter + promise + ledger + two evaluation/metrics).

## Findings

The diagnosis proper is accurate and I found no incorrect claim. Findings are
about the plan.

### 1. issue: the live defect is left broken for the duration of the largest rewrite

The plan orders work as "port required behavior first in reviewable commits,
then make the observation fixes in separate commits before final acceptance".
The diagnosed defect is a one-record timestamp failure that has failed the
observer on every run since 15:36Z (37 failures sampled) and blocked the hourly
hook measurement. Under this ordering it persists until a 24-file port (six
scripts, six test files, 12 skill helpers, inline Python) lands, and
the same port's tests are what the plan then uses to "reproduce the real
cancelled-job timestamps". Nothing in the plan states the interim state or
accepts it. Either make the timestamp/504 fix the first change (in the file as
it stands, or as the first Go port of `release_timing`, ahead of the rest), or
say explicitly that the observer stays red until the port completes. The first
is also what "what is broken right now" calls for.

### 2. issue: the hook decision makes smoke tests absent on every fresh worktree

The decision (Bash launcher + Go helper built only by `just warm-hook` into
local Git metadata, never built or `go run` in the hook) means that in any
worktree or clone where `warm-hook` has not been run, the smoke tests do not
execute and the hook prints the budget warning for every commit. This
repository is routinely worked in fresh worktrees (this one is under
`~/.codex/worktrees/`); Git metadata is per-worktree, so each needs its own
warmup. Today, `pre-commit.py` runs `go test` for four packages, the Node
test, and the dispatch test within 2.7 s on a warm cache with no helper.
So the port converts "smoke checks run, warn only if slow" into "smoke checks
are skipped unless a manual step was done", which AGENTS.md permits only for a
slow check, not for a structurally missing one. Decide how a fresh worktree
gets the helper without compiling in the timed path (for example built once by
a `post-checkout`/`post-merge` lefthook job or at first `just` use, or by
keeping the helper source identity per Go build cache so a cache-hit build is
within budget), and state the expected steady-state, since this is the gate
agents see on every commit.

### 3. issue: skill-helper scope is decided unilaterally and its cost is unexamined

The plan now removes all twelve `.agents/skills` helpers. That answers round
03's ambiguity, but the answer is the plan author's, not Tyler's: the
instruction recorded is "remove Python", with only the replacement language
delegated. Those helpers include borrowed/synced skill content (see
`update-borrowed-skills`), a file-lock-based promise store (`fcntl`), and an
evaluation harness. Porting them is a separate and large behavioral rewrite
(locking semantics, YAML parsing equivalence across seven copies, how a skill
invokes a Go program: `go run` per call versus a built binary). The plan lists
no behavior-preservation proof for them, unlike the six release scripts.
Either get Tyler's confirmation of scope, or add a proof item for the helpers
and record how future borrowed-skill syncs avoid reintroducing Python.

### 4. nitpick: round-03 attribution

The plan says "Its claim of a Python .gitignore entry was checked: none is
present." Round 03 did not claim an entry exists; it listed `.gitignore` /
`scripts/__pycache__` among places to inventory. The statement of fact (no
entry; untracked `scripts/__pycache__` should be deleted) is right, the
attribution should be softened.

### 5. nitpick: the closing sentence

"Final review of this revision is pending" should be replaced with this
round's outcome and adjudication.

## Outcome

material findings remain
