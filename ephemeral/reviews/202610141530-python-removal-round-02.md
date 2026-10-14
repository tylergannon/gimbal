# Adversarial review: Python removal, round 02

## Target

Current working tree of `codex/remove-python` against `origin/main`
(116 files, +10489/-6023), measured against `diagnosis.md` (remediation plan),
`execution.md`, `AGENTS.md`, and Tyler's authorization to execute the whole plan.

No caller instruction narrowed the scope.

## Evidence inspected

- The full `git diff origin/main --name-only` list, with no untracked files left:
  the replacement sources are now in the diff.
- Workflows, `Justfile`, `lefthook.yml`, `scripts/pre-commit.sh`,
  `scripts/check-automation.sh`, `servicecheck.go` and its generated definition,
  and the `gimbal-runs` / `gimbal-release` skill text.
- A repository-wide, case-insensitive search for `python`, excluding
  `ephemeral/`, `node_modules` and `.git`.
- I ran `bash scripts/check-automation.sh`: automation and 12 skill modules passed
  tests, vet and lint, and the inline-Python grep guard passed.
- I ran `node --test scripts/*.test.mts`: 21 of 21 passed.
- I ran `go test ./internal/workflows/servicecheck/`: passed, including the
  new test file.
- I did not run the GitHub or Mac runners or a real release (plan proof items 3
  and 4), so those remain unverified here.

## Round-01 finding status

- **Inline Python in `service-check`: resolved.** `servicecheck.go` now uses
  `node --input-type=module -e` for the web server and fetch consumer. The
  prerequisite text, generated definition (snapshot hash changed), and
  `skills/gimbal-runs/SKILL.md` all say Node.
- **`check-automation.sh` guard: resolved.** It now greps tracked Go, shell and
  TypeScript for `python... -c/-m` as well as rejecting tracked `.py` files.
- **Untracked replacements: resolved.** All replacements appear in the diff.

## Findings

No critical or issue-level findings.

### 1. [nitpick] The no-Python poison step is repeated

The same 7-line block appears eight times in the three workflows (verified
by grep: `auto-release.yml` 2, `release.yml` 4, `release-performance.yml` 2,
eight in total). It works and each job needs it, but a copy edited alone drifts.

### 2. [nitpick] A live-looking Python command remains in a sprint document

`docs/sprints/SPRINT-002.md:19` says to rerun with `python3 .../run.py`. It is a
historical record of a tmp script, not maintained automation, and the plan
exempts historical notes. Only worth a note because `docs/` is not under
`ephemeral/`.

## Outcome

only nitpicks remain
