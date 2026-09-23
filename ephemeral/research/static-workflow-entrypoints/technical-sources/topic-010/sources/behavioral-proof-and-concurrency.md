# Behavioral Proof Standards, Concurrency Verification, and Live Model Policy

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble`
- **Files**:
  - `AGENTS.md`
  - `ephemeral/research/static-workflow-entrypoints/definition-of-done.md`
  - `docs/definition-of-done.md`
- **Commit/Baseline**: `main` / `e161721c`
- **Retrieval Date**: 2026-09-23

---

## 1. Owner's Rules on Proof and Artifacts in `AGENTS.md`

From `AGENTS.md` (lines 40–56, 70–73):

```markdown
## Proof is what you saw

Proof is running the real thing yourself and saying what you saw, in the
chat or the PR description. There are no proof programs. Nothing written
to perform or record a run is committed: no `ephemeral/attest/`, no
`result.md`, no screenshots, no run logs, no text dumps of a run. A check
that should be repeatable is a test in the package it checks. An issue's
"Proof" section is satisfied by that report.

Apart from the frozen old code in `ephemeral/legacy/`, `ephemeral/` holds
notes, never code: nothing there is built, vetted, or maintained. The
commit hook refuses new code and run output under it.
...
- Live runs you start to see something work use the cheapest models:
  Codex `gpt-5.6-luna`, Claude Haiku, Gemini flash. Say which model a run
  used.
```

---

## 2. Multi-Project Concurrency and Host Lifecycle Requirements

From `ephemeral/research/static-workflow-entrypoints/definition-of-done.md` (lines 80–105):

```markdown
## One host, first-use projects, and retained lifecycle

- Starting a workflow for a valid project unknown at startup admits that
  project automatically. No preliminary registration command or server restart
  is necessary. Concurrent first requests for the same canonical project reuse
  one project owner; an ordinary path alias does not create another owner.
- One serving PID owns overlapping runs in different projects. Project A's
  pages and controls cannot address project B's runs. Execution workdirs remain
  distinct from the owning projects' durable storage locations.
- Closing the browser, exiting the launch CLI, or stopping follow leaves the
  accepted run active. Explicit cancellation affects the intended run. Existing
  failure recording, hosted panic containment, coordinated shutdown, project
  ownership release, and history after restart remain working.
- The same generated remote/client path works with `--no-web`. An absent host
  or unavailable remote produces an actionable error; no second runtime owner
  or local execution fallback is started. Explicitly selected independent
  instances remain usable for separate projects and tests.

Evidence: start one built instance with project A, then launch into previously
unknown B through a separate CLI and through a browser start form as applicable.
Observe overlapping work, the single serving PID, correct project pages, and
continued work after the initiating client leaves. Reopen completed histories
after restart. Use existing ownership/lifecycle tests for deterministic alias,
concurrent-admission, failure, and panic cases, extended for the new entry path.
Repeat a representative launch headlessly through the generated Go client.
```

---

## 3. Delivery and Evidence Quality Standards

From `ephemeral/research/static-workflow-entrypoints/definition-of-done.md` (lines 106–122):

```markdown
## Delivery and evidence quality

Use cheap live model bindings: Codex `gpt-5.6-luna`, Claude Haiku, or Gemini
flash. Record which model each observation used. Choose small valid inputs and
isolated projects; do not operate on unrelated active runs to prove lifecycle
behavior.

SKGO's relevant checks and Gimble's required build, test, vet, formatting, and
browser checks pass. In Gimble these are `just build`, `just test`, `just vet`,
`just fmt-check`, and `just e2e`. Inspect generated output and the actual
rendered CLI help. These are correctness gates separate from behavioral proof.

An independent validator confirms that the observations establish the claims
above. The implementation report identifies the tested source revision,
observed outcomes, and any unmet requirement. A mocked endpoint or aggregate
green gate does not establish shared remote operation or single-process live
ownership.
```
