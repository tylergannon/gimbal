# Date extraction

## Purpose

This cookbook extracts the semantic parts of absolute and relative dates with seven Jev `Choice` questions, then performs calendar arithmetic, validation, inference, and review routing in deterministic code. It is a compact example of reserving Jev for language understanding while keeping computable invariants out of the model.

## Key concepts and evidence

- Jev identifies mode and named parts; it never performs calendar math. Code converts the choices into a `date`, validates impossible combinations, and resolves relative language against a pinned `TODAY`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:9-25`
- Seven Choices are sent in one call: mode, month, day, year, relative anchor, weekday, and week offset. Code reads only the parts relevant to the chosen mode. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:99-175`
- The year taxonomy has 151 explicit years plus `none` and `out_of_range`; `none` permits deterministic inference, while `out_of_range` flags instead of guessing. The cookbook suggests extracting year-like numbers first if this option set is undesirable. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:111-114` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:138-149`
- Overall confidence is the minimum confidence among only the parts actually used. An unassemblable or below-`0.60` result goes to review. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:178-188` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:215-232`
- Ambiguous conventions are policy, not model output: bare weekday means next occurrence on/after today, `next` means following calendar week, and `current` means this week. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:186-188` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:203-212`

## Measured examples

- Six extraction tasks all matched expected output: two explicit contract dates (`0.97`, `0.91`), one yearless deadline (`0.95`), “today” (`0.94`), “next Thursday” (`0.92`), and one absent date returned none at `0.46` and was flagged. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:298-345`
- Code inferred 2026 for an August 14 date without a year and resolves `next Thursday` as 2026-08-06 from pinned Thursday 2026-07-30. The absent kickoff date produced `absolute date incomplete`, not a hallucinated concrete date. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:347-355`
- Five results auto-accepted and one went to review under the `0.60` minimum-part threshold. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:357-395`

## Citation bookmarks

- Architecture: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:5-31`
- Question schema: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:99-175`
- Deterministic resolver and failure cases: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:178-295`
- Six-result scorecard: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/date_extraction_cookbook.md:298-355`

## Themes for continuous supervision

- **Semantic parse plus deterministic executor:** Jev reads language; Go resolves time windows, durations, attempt counts, deadlines, and impossible states.
- **Ask all parts, consume conditionally:** batching avoids sequential latency while code guards against irrelevant companion answers.
- **Weakest-link confidence:** compound interventions should inherit risk from their least-certain required component.
- **Explicit conventions:** phrases such as “stalled”, “soon”, or “try again” need code-owned temporal/action semantics once Jev identifies their linguistic category.

## Gotchas and failure modes

- Seven Choices are asked even though most are irrelevant for any one date; downstream code must never treat unused companion answers as evidence.
- The 1900-2050 year list is a brittle closed world. Deterministically extracting numeric candidates first is safer and smaller.
- The absent-date example still chose `absolute` and was caught only because required parts were missing. Validation code is therefore essential; mode classification alone can false-positive.
- The cookbook calls confidence calibrated, but six handpicked examples do not demonstrate calibration. The `0.60` gate is an illustrative policy.
- Relative-date conventions vary by locale and organization. Bigger models or humans may be needed for genuinely ambiguous phrasing, but final calendar math should remain deterministic.

## Task recipes

1. **Extract supervision deadlines:** ask Jev for urgency mode and named temporal parts from user/agent text; use Go time libraries to compute deadlines, reject impossible windows, and flag low minimum-part confidence.
2. **Parse retry/stop instructions:** model “after N failures”, “until tomorrow”, and “next review” as semantic parts; keep counters, clocks, and state transitions deterministic.
3. **Test absence aggressively:** include documents containing nearby but role-incorrect dates/times. Require validation to distinguish “a date exists” from “the requested date exists.”
