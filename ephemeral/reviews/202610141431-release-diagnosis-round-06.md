# Adversarial review: release diagnosis and remediation plan, round 06

## Target

The entire current `ephemeral/research/release-diagnosis/diagnosis.md` (372
lines): diagnosis, evidence, adjudications of rounds 01-05, and the plan with
its new Node floor, runtime-prerequisite handling, and test-gate decision.
Judged against the recorded request, `AGENTS.md`, source, and available
evidence.

The launch prompt supplied only target, artifact path, and read-only limits;
nothing narrowed scope. (The file name's 1431 is caller-supplied.)

## Evidence inspected

- The whole plan text and the round-05 adjudication.
- Round-05 findings against the plan: nested skill modules and Node hook tests
  now have an explicit gate (`go -C <skill> test ./...`,
  `node --test scripts/*.test.mts`); the Node floor is declared; both resolved.
  The decision not to adopt my "advisory skip on old Node" proposal is sound:
  `scripts/pre-commit.py` already turns `OSError`/`HookFailure` (a missing or
  unusable `node` for the smoke command) into exit 1, and AGENTS.md says only
  elapsed-budget cases are advisory. Accepted.
- `Justfile:48-68`: the hook-test target is named `check-hooks`
  (lines 59-61); `warm-hook` at 64, `benchmark-hook` at 68.
- Installed Node toolchains on this machine: Homebrew `node@24` 24.21.0,
  Homebrew `node` 26.11.1, Vite+ 24.22.0. No 24.13.0 anywhere (`find` for
  `*24.13.0*/node` returned nothing).
- Probe I ran: a `.mts` file with a type annotation and a `node:path` import
  executes on Node 24.21.0 with exit 0 and empty stderr (output `b 1`), as
  it does on 26.11.1.

## Findings

No incorrect claim about the release diagnosis and no unresolved plan gap
that I consider material. Remaining items are accuracy and wording.

### 1. nitpick: the 24.13.0 probe claim cannot be reproduced from this machine

The plan says "A local probe on the exact installed Node 24.13.0 executed type
annotations and a Node standard-library import successfully with empty
stderr". No Node 24.13.0 is installed here; the installed 24.x builds are
24.21.0 and 24.22.0. AGENTS.md's rule is that proof is what was seen and
reported, so this sentence should name the version actually exercised
(or the probe should be repeated on 24.13.0 from a downloaded build).
My probe on 24.21.0 supports the substance (no warning, no flag), so the floor
is likely safe, but the floor itself (`>=24.13.0`) is then justified only by
that unverified probe; say why 24.13.0 rather than the lowest Node 24.x
that strips types without a warning.

### 2. nitpick: wrong Justfile target name

The plan says "Justfile's hook-check target runs the hook tests". The target
is `check-hooks` (`Justfile:59`). Use the real name so the instruction is
greppable.

### 3. nitpick: ambiguous phrase

"reject an unsupported source-commit runtime" is unclear (source-commit
runtime = the runtime needed for commits that touch source?). Say "a commit
that changes source files" once, since the lightweight documentation-only path
is the distinction the sentence is trying to preserve.

### 4. nitpick: closing sentence

"Final review of this revision is pending" should be replaced with this
round's outcome. Also, as with prior rounds, the section's adjudication
accounts for each earlier finding; nothing is missing there.

## Outcome

only nitpicks remain
