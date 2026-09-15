## Implementation Plan: slug() Function

**Requirement Analysis**

The `slug()` function must:
1. Strip leading/trailing whitespace
2. Convert to lowercase
3. Replace each run of consecutive whitespace with a single hyphen
4. Preserve all other characters (punctuation, existing hyphens, unicode)
5. Return empty string for empty or whitespace-only input

**Test Coverage** (12 cases from check.py):
- Empty and whitespace-only strings: `""`, `" "`, `"\t\n"` → `""`
- Basic lowercase: `"Hello"` → `"hello"`
- Whitespace replacement: `" Hello World "` → `"hello-world"`, `"a\tb\nc"` → `"a-b-c"`
- Multiple consecutive spaces: `"A   B"` → `"a-b"`
- Punctuation preservation: `"Keep! Punctuation?"` → `"keep!-punctuation?"`
- Existing hyphens preserved: `"a--b"` → `"a--b"`
- Digits unchanged: `"123"` → `"123"`
- Unicode lowercase: `"ÉCOLE"` → `"école"`
- Adjacent hyphens from spaces: `"x - y"` → `"x---y"` (space→hyphen, dash stays, space→hyphen)

**Algorithm**

1. Strip leading/trailing whitespace with `.strip()`
2. Check if result is empty; return `""` if so
3. Convert to lowercase with `.lower()`
4. Use regex substitution: `re.sub(r'\s+', '-', text)` to replace each run of whitespace with a single hyphen
5. Return the result

**Implementation Tasks**

| # | Task | Validation |
|---|------|-----------|
| 1 | Create `slug.py` with `slug(text: str) -> str` function | File exists in workspace |
| 2 | Implement strip + empty-check logic | Test cases 1–2, 11 pass |
| 3 | Implement lowercase conversion | Test cases 3, 10 pass |
| 4 | Implement whitespace-to-hyphen replacement using `re.sub()` | Test cases 4–7, 9, 12 pass |
| 5 | Verify hyphens and special characters are preserved | Test cases 8–9 pass |
| 6 | Run `python3 check.py` and confirm all 12 cases pass | Output: `SPRINT_SLUG_OK: 12 cases` |

**Key Decisions**
- Use `re.sub(r'\s+', '-', text)` for robust handling of all whitespace types (space, tab, newline)
- Order operations: strip → check empty → lowercase → regex substitute (maintains unicode correctness and hyphen preservation)
- No additional dependencies beyond Python's built-in `re` module
