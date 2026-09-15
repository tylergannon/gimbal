# Implementation Plan: Slug Function

This implementation plan synthesizes the drafts and resolves critique points. It preserves the exact goal, acceptance criteria, constraints, and local context paths without modifying any files or running mutating commands during planning.

---

## 1. Goal & Acceptance

- **Goal**: Implement the `slug` function described in [`brief.md`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md).
- **Acceptance Criteria**: All 12 fixed [`check.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py) cases pass with only `slug.py` added.

---

## 2. Constraints

- Do not implement during planning.
- Preserve [`check.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py) and [`brief.md`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md) exactly as they are.
- Only create [`slug.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/slug.py); no external dependencies/packages, CLI, documentation, git commits, or remote actions.

---

## 3. Critique Resolutions & Synthesis

1. **Exact Algorithm & Execution Order**:
   - Stripping leading/trailing whitespace must happen before collapsing whitespace to avoid creating leading/trailing hyphens.
   - Sequence:
     1. `stripped = text.strip()`
     2. `if not stripped: return ""`
     3. `lowered = stripped.lower()`
     4. `re.sub(r"\s+", "-", lowered)` (or `"-".join(stripped.lower().split())`)
   - Standard library `re` handles all whitespace runs (`\s+`), Unicode lowercasing (`"ÉCOLE"` -> `"école"`), and preserves existing hyphens and punctuation (`"x - y"` -> `"x---y"`).

2. **Validation Integrity Scoping**:
   - Critiques noted that the wider workspace has pre-existing uncommitted files, so a generic repository-wide status check is flawed.
   - Cleanliness validation must be scoped explicitly to the task directory: verify that [`brief.md`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md) and [`check.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py) have unchanged checksums/content and that `slug.py` is the only newly created file in `/Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace`.

3. **Atomic vs. Granular Tasks**:
   - Because `check.py` is an all-or-nothing test script, implementation will be done directly in `slug.py` rather than claiming artificial intermediate test passes.

---

## 4. Ordered Tasks

1. **Create [`slug.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/slug.py)**
   - In `/Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/slug.py`:
     - Import `re` from Python standard library.
     - Define `slug(text: str) -> str`.
     - Strip leading and trailing whitespace with `text.strip()`.
     - Lowercase the stripped string with `.lower()`.
     - Substitute runs of whitespace characters using `re.sub(r"\s+", "-", ...)`.
     - Return the resulting string (empty strings naturally return `""`).

2. **Execute Functional Validation**
   - Run:
     ```bash
     python3 /Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py
     ```
   - Verify exit status is `0`.
   - Verify stdout contains: `SPRINT_SLUG_OK: 12 cases`.

3. **Verify Integrity & Constraints**
   - Verify [`brief.md`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md) and [`check.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py) are unchanged.
   - Verify that within the plan workspace directory (`/Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace`), only `slug.py` has been introduced.

---

## 5. Demonstration & Evidence

- **Execution Command**:
  ```bash
  cd /Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace && python3 check.py
  ```
- **Expected Output**:
  ```text
  SPRINT_SLUG_OK: 12 cases
  ```
- **Scoped Verification**:
  ```bash
  git diff --stat -- /Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md /Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py
  ```
  (must show 0 changes)
