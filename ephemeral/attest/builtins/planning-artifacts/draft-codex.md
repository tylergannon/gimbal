Read the repository instructions, `brief.md`, and `check.py`. No files were changed.

Plan:

1. Add only `slug.py` with `slug(text: str) -> str`.
2. Implement the exact contract using Python standard-library string behavior:
   - collapse each run of whitespace into `-`;
   - trim surrounding whitespace;
   - lowercase;
   - preserve punctuation and existing hyphens;
   - return `""` for empty or whitespace-only input.
3. Run `python3 check.py`.
4. Confirm the output is `SPRINT_SLUG_OK: 12 cases`, and verify `check.py` and `brief.md` are unchanged and no files besides `slug.py` were added.

No packages, CLI, documentation, commits, or remote actions are needed.
