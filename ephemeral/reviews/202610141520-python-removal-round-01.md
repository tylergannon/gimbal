# Adversarial review: Python removal, round 01

## Target

Working tree of `codex/remove-python` at `3bd35796` (equal to `origin/main`),
reviewed against `ephemeral/research/release-diagnosis/diagnosis.md`
(remediation plan), `execution.md`, `AGENTS.md`, and Tyler's authorization to
execute the whole reviewed Python replacement plan.

No caller instruction narrowed the scope.

## Evidence inspected

- `git diff origin/main` (49 tracked files, +363/-6002) plus the 26 untracked
  replacement files: `internal/automation/*`, the skill `scripts/*/` Go modules,
  `scripts/pre-commit.{sh,mts}`, `scripts/check-automation.sh`, and the `.mts` tests.
- The three workflows, `Justfile`, `lefthook.yml`, `generate-appcast.sh`,
  `release-macos.sh`, `local_service_fulfillment_test.go`, and docs/skill edits.
- A repository-wide, case-insensitive search for `python`, `.py` and `venv`
  outside `ephemeral/` and `node_modules`.
- I ran `bash scripts/check-automation.sh`: automation tests, vet, lint, and
  all 12 skill modules passed. I ran `node --test scripts/*.test.mts`: 21 of 21 passed.
- I did not run the GitHub runners, the Mac signing path, or a real release
  (plan proof items 2-4), so none of that is verified here.

## Findings

### 1. [issue] A maintained workflow still executes inline Python

Plan requirement: "The maintained automation has no executable Python
references", and "delete the old Python files and runtime requirements".

`internal/workflows/servicecheck/servicecheck.go` still runs `python3`:

- The header comment at line 6 lists "Python 3" as a prerequisite.
- `gimbal.RunCommand(..., "python3", "-c", consume)` is at line 60.
- A `python3 -c 'import http.server...'` web service command is the `web`
  constant at line 90.
- The generated `internal/invocation/servicecheck_gen.go` also mentions python.
- `skills/gimbal-runs/SKILL.md:234` tells users that `service-check` needs
  Python 3.

The worklog correction found the same inline-Python pattern in
`local_service_fulfillment_test.go` and fixed it with the Go test executable.
The same fix was not applied to this sibling. The Python-free guarantee therefore
does not hold for the repository: running the `service-check` workflow on a
machine without Python fails.

Why the gate missed it: `scripts/check-automation.sh` only rejects tracked
`*.py` files, and the PATH poison covers only the shell steps that run it. Inline
`python3 -c` in Go source is invisible to both. The `check-automation.sh` guard
is a filename check, not the no-executable-Python check the plan describes.

Impact: an incomplete requirement. The Python runtime prerequisite remains in
user-facing documentation and in a shipped workflow.

Fix direction: replace both snippets with Go (a `RunCommand` of the Go test
binary, or in-process HTTP and fetch). Regenerate the `_gen.go` file and update
the SKILL.md prerequisite. Make the guard also grep `python3?` in tracked
non-`ephemeral/` source.

### 2. [nitpick] Replacements are untracked, so the diff does not show them

The 26 replacement sources and tests are `??` (untracked). A
`git diff origin/main` shows deletions only; the ported behavior is not in the
diff reviewers or CI would see until it is staged. The deletions are unreviewable
as a port until the files are added. Add them when committing.

### 3. [nitpick] The no-Python poison step is copied eleven times

The same 7-line shell block appears in all three workflows (11 occurrences in
total). A repeated step is acceptable under "no wrappers", but a one-line
`bash scripts/...` call, or `check-automation.sh` reuse, would be shorter and
cannot drift.

## Items checked and found sound

- The old Python entry points, their tests, the skill helpers, `appcast` and
  notary inline Python, and the Mac `python3` preflight are all removed or replaced.
- `pre-commit.sh` keeps the documentation-only path independent of Node and
  rejects Node <24.13.0 for source commits as an actual failure, as required.
- `.mts` is added to the ephemeral-code refusal pattern.
- Skill SKILL.md files invoke `go run -C "$HELPER"` against a self-contained
  module beside the skill, with no dependency on the Gimbal checkout.

## Outcome

material findings remain
