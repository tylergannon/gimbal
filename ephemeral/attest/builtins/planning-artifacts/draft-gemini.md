# Implementation Plan: Slug Function

This plan outlines the steps to implement the `slug` function per [brief.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md) while satisfying all 12 test cases in [check.py](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py). In accordance with the constraints, no files have been modified and no mutating commands have been executed.

---

## Proposed Design & Algorithm

Implement `slug(text: str) -> str` in a new file [`slug.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/slug.py):
1. **Strip & Check Empty**: Strip leading/trailing whitespace with `text.strip()`. If the stripped string is empty, return `""`.
2. **Normalize Case**: Lowercase the string via `.lower()`.
3. **Collapse Whitespace**: Replace any contiguous run of whitespace characters (`\s+`) with a single hyphen `-` (e.g. using `re.sub(r"\s+", "-", stripped.lower())`).
4. **Preserve Characters**: Retain all punctuation, unicode characters, and pre-existing hyphens untouched (as demonstrated by `"x - y" -> "x---y"` and `"ÉCOLE" -> "école"`).

---

## Ordered Tasks

1. **Create [`slug.py`](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/slug.py)**
   - Import the standard library `re` module.
   - Define `slug(text: str) -> str`.
   - Strip leading and trailing whitespace using `text.strip()`.
   - Lowercase the result using `.lower()`.
   - Substitute any whitespace runs (`r"\s+"`) with `-` using `re.sub`.
   - Return the transformed string.

2. **Validate Against Acceptance Criteria**
   - Execute `python3 check.py` in the workspace directory.
   - Verify that all 12 assertion cases pass without errors and the output is `SPRINT_SLUG_OK: 12 cases`.

3. **Verify Constraints & Cleanliness**
   - Confirm only `slug.py` was created.
   - Confirm [brief.md](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/brief.md) and [check.py](file:///Users/tyler/.codex/worktrees/7b5c/gimble-builtins/ephemeral/attest/builtins/plan-workspace/check.py) are unchanged (`git status`).

---

## Concrete Validation

- **Command**:
  ```bash
  python3 check.py
  ```
- **Expected Result**:
  - Exit code: `0`
  - Output: `SPRINT_SLUG_OK: 12 cases`
- **Integrity Check**:
  ```bash
  git status --porcelain
  ```
  - Expected output: Only `?? slug.py` untracked, with no modifications to existing files.
