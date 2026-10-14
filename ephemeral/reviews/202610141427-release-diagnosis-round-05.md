# Adversarial review: release diagnosis and remediation plan, round 05

## Target

The entire current `ephemeral/research/release-diagnosis/diagnosis.md` (340
lines): diagnosis, evidence, adjudications of rounds 01-04, and the revised
plan (observer-first ordering, TypeScript hook coordinator, skill-helper Go
ports with local module context). Judged against the recorded request,
`AGENTS.md`, and source/evidence.

The launch prompt supplied only target, artifact path, and read-only limits;
nothing narrowed scope.

## Evidence inspected

- Whole plan text and all adjudications.
- Round 04 findings, checked against the plan: observer-first ordering is now
  stated (finding 1 resolved); fresh-worktree skipped smoke is resolved by
  removing the compiled helper (finding 2 resolved); skill proof item 5 was
  added (finding 3 resolved).
- Local `node --version` = v26.11.1; a `.ts` file with a type annotation runs
  directly with no flags (0.3 s wall including cold start). CI pins Node 24
  (`auto-release.yml:57`, `release.yml:35,127,185`, `release-performance.yml:46`).
- `web/package.json`: no `engines` field. `.agents/skills/` contains no
  `go.mod` or provenance file today.
- Earlier-round facts (hook semantics in `scripts/pre-commit.py`, `lefthook.yml`,
  Python inventory) unchanged.

## Findings

The diagnosis remains accurate. The revision addresses all round-04 issues
substantively. Remaining defects are in plan completeness.

### 1. issue: nested skill Go modules and Node hook tests fall outside the test gate the plan names

The plan replaces the Python gate with "the affected Go package tests" and
says "do not remove coverage". Two new homes for tests escape that gate:

- Skill helpers are to live in "a small Go module context" beside each
  `SKILL.md` with their own pinned yaml.v3. A nested `go.mod` is excluded from
  the root module's `./...`, so `go test ./...`, `go vet`, and golangci in
  `auto-release.yml` will never see them. Proof item 5 is a one-time manual
  fixture exercise, not a repeatable test, and AGENTS.md says a repeatable check
  is a test in the package it checks and that CI is what validates.
- The six Python test files include `staged_hook_test.py` and
  `fast_hook_test.py`, which the plan moves to "Node tests beside the
  TypeScript hook". No workflow step or Justfile target is named that runs
  them. Today `auto-release.yml:62` and `Justfile:60-61` run them.

State where each runs (e.g. explicit `go test` with `-C` per skill module in
the Go job; `node --test scripts/*.test.ts` in the Node job and
`just warm-hook`), otherwise coverage silently drops at the first green CI.
Seven copies of the model-resolver as seven modules also repeats the existing
duplication; if they are meant to stay copies, say so.

### 2. issue: the hook's Node requirement is asserted, not declared, and its failure mode blocks commits

"Already-required Node 24" is true only of CI workflow pins; nothing in the
repo (no `engines`, no `.nvmrc`) declares it, and developers/agents run other
majors (here 26). Direct TypeScript execution depends on a Node new enough to
strip types by default and on erasable-only syntax (no enums, parameter
properties, `import` of non-`.ts` specifiers without extensions). On an older
Node the coordinator is a syntax error: a nonzero exit from the mandatory
path, which the hook treats as a blocking failure, not as "unfinished smoke".
Specify: the launcher checks `node --version` and, if unsupported, runs the
mandatory staged checks and reports smoke unfinished (consistent with the
existing advisory rule), and the minimum version is declared where tooling can
read it. Also confirm on Node 24 specifically that type stripping prints no
warning on stderr that the hook output or timing report would capture; I could
only verify on 26.

### 3. nitpick: three automation languages for one concern

After the plan, automation is Bash launcher + TypeScript hook + Go
release/CI/skill programs. Each choice is individually justified (Node is
already present; Go matches the repo), but "as simple as possible" would
favor one program set per concern with the split stated once ("hook: Node;
everything else: Go") so later contributors don't add a fourth style. The plan
effectively says this; make it a single sentence.

### 4. nitpick: the round-04 adjudication dismisses scope confirmation

"No further permission question is needed" is defensible given the recorded
instruction. Note that the skill ports are by far the largest and riskiest
part of the plan; sequencing them last (as written) and noting they can be
dropped from the first delivery without affecting observer recovery would
keep that decision cheap to revisit.

### 5. nitpick: closing sentence

"Final review of this revision is pending" should be replaced with this
round's outcome and adjudication.

## Outcome

material findings remain
