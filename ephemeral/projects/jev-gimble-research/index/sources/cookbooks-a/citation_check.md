# Double-checking citations

## Purpose

This cookbook verifies a claim-and-citation pair in two stages: deterministic quote location first, then one Jev `Choice` over the located section to distinguish support, contradiction, and silence. It is a direct template for supervising research agents and evidence-bearing run reports at high volume.

## Key concepts and evidence

- Exact/folded string matching handles a fabricated quote without a model; a cited section with no quote proceeds directly to semantic checking. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:157-207`
- The semantic question has three mutually exclusive relations: `supports`, `contradicts`, and `says_nothing`, mapped in code to `verified`, `contradicted`, and `unsupported`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:220-277`
- A confidence threshold of `0.8` decides whether the semantic verdict stands or a human confirms it; fabricated quotes are auto-routed without model confidence because no Jev call occurred. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:220-279`
- The cookbook explicitly recommends starting with a high human-review threshold and lowering it only after observing performance on the target documents. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:220-231`

## Measured examples

- Corpus: RFC 7519, 58,365 characters, 45 numbered sections, eight test citations; four citations were accurate and four were intentionally corrupted. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:91-155`
- All four accurate citations were verified with confidence at least `0.93`. A missing quote was marked fabricated deterministically. A word-for-word quote supporting the opposite proposition was marked contradicted at `0.99`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:282-328`
- Two unsupported citations landed at confidence `0.27` and `0.56`, below the `0.8` threshold, so they went to human review. One demonstrates that literal quote presence is insufficient when the surrounding section says nothing about the derived claim. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:301-328`

## Citation bookmarks

- Two-stage architecture and four outcomes: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:5-55`
- Source normalization and section location: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:91-207`
- Semantic relation and review policy: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:210-280`
- Full result table and interpretation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:282-332`

## Themes for continuous supervision

- **Deterministic prefilter, semantic verifier:** resolve file existence, quoted spans, exit codes, and artifact identity in code before Jev evaluates whether evidence supports a claim.
- **Contradiction is first-class:** do not collapse “contradicts” into “not supported”; a supervisor may need to interrupt or redirect when the run’s own evidence disproves its current plan.
- **Claim-local state:** send the exact claim and relevant source section, not a whole repository or unbounded transcript.
- **Human review is a policy outcome:** uncertainty should create review work rather than a forced binary verdict.

## Gotchas and failure modes

- The exact-normalized match rejects truncated or lightly paraphrased quotes as fabricated. A production verifier that permits loose quotation needs fuzzy matching or span retrieval before Jev. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:324-332`
- The RFC-specific section parser must be replaced for other source shapes. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/citation_check.md:324-329`
- The eight examples are planted, not an independent benchmark; the `0.8` threshold is a starting policy, not evidence of calibrated production risk.
- A found quote may be contextually irrelevant or support the opposite claim; string matching alone creates dangerous false positives.
- Jev does not retrieve the authoritative source here. Retrieval, source pinning, document parsing, and quote matching remain deterministic/external responsibilities.

## Task recipes

1. **Verify agent proof claims:** parse each “claim → artifact/citation” pair; verify referenced file/run/event exists and locate the exact span in code; ask Jev whether the span supports, contradicts, or does not address the claim; auto-accept only above a calibrated threshold.
2. **Continuous run-plan consistency:** compare the worker’s current assertion (“tests now pass”, “blocked on user input”, “review defect resolved”) with the most relevant recent event span; route contradiction to immediate steering, silence to evidence request, and low confidence to agent/human review.
3. **Avoid false-positive proof:** distinguish `fabricated`, `unsupported`, `contradicted`, and `verified` in UI and downstream policy; never map all non-verified states to a single generic warning.
