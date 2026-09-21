# Cascades and bounded extraction

## Purpose

This leaf covers two ways to avoid asking Jev to generate values: select verbatim values from candidates, or use Jev as a cheap semantic verifier between a fast model and an expensive reasoning model.

## Key concepts and measured results

- **Pre-parsed extraction separates finding, choosing, and normalization.** Regex or another candidate generator over-finds spans, Jev selects the span and semantic attributes, and deterministic code copies/normalizes it. Because the Choice options are the original spans, Jev cannot transpose a digit or invent a new value. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:5-27`
- **Every candidate selection needs a `none` escape hatch.** The helper builds a Choice over found spans plus “none of these,” while separate Choice/Noul questions classify attributes such as country, currency, or credit-versus-charge. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:53-74`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:78-138`
- **Three worked examples demonstrate exact-copy behavior.** Email selection chose a personal receipt address at confidence `0.98` and sender at `1.00`; phone selection chose the mobile at `1.00` and country at `0.90` before code formatted E.164; invoice selection chose total/credit and Noul assigned credit probabilities `0.01` and `0.99`. These are demonstrations, not corpus-level accuracy results. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:140-179`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:181-218`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:220-276`
- **Candidate generation is the hard boundary.** Choice is limited to 255 candidates; larger sets require staged narrowing. Values such as names need a roster, NER system, or generative model to propose candidates before Jev can select. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:306-314`
- **The SDE cascade is `cheap extract → Jev verify → expensive re-extract`.** Per-field Nouls estimate whether something is wrong; if any signal crosses a threshold, the pipeline escalates to a stronger reasoning model, otherwise it keeps the cheap answer. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:5-38`
- **Schema validity is not semantic correctness.** The cheap model produced a schema-valid record with a fabricated description. The example deliberately hard-codes one observed fabrication because the mini model was stochastic; Jev later flags hallucination and off-target extraction. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:253-338`
- **Verifier questions are decomposed per field and failure mode.** Non-empty fields get checks for name/description mismatch, type mismatch, unreasonableness, hallucination, off-target extraction, incompleteness, and format violation. Empty fields get an absence-wrong check. A whole-record judge is displayed for comparison but intentionally excluded from the gate. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:340-359`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:361-508`
- **The worked verifier localizes its strongest signals.** `description::hallucinated=0.95` and `description::off_target=0.85` exceed the `0.7` gate, while the holistic judge is only `0.56` and the correct absent field's `absence_wrong` is `0.14`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:511-549`
- **Aggregation uses max/any, not an average.** A single confident per-field red flag escalates; averaging could bury a sparse severe error among many low signals. The reasoning model then removes the fabricated field. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:551-612`
- **The 100-prompt cascade result is internal and partly qualitative.** The page says its threshold sweep produced a Pareto frontier up-and-left of four standalone models, with the strongest model at roughly `0.81` quality and `$0.10/extraction`; it also says the chart is historical and costs were not recalculated to current Jev pricing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:614-634`
- **Good verifier signals are narrow, grounded, failure-positive, independent, and separating.** The cookbook recommends “bad = true,” explicit true/false criteria, per-field localization, max aggregation, and a cheap verifier distinct from the extractor. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:636-658`

## Important citation bookmarks

- Candidate-first extraction contract: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:5-27`
- Candidate selection helpers: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:67-138`
- Candidate/Choice limits: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:306-314`
- SDE algorithm and model/cost snapshot: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:5-38`
- Per-field verifier construction: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:340-508`
- Worked signals and escalation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:511-612`
- Internal 100-prompt claim and caveat: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:614-634`

## Themes

- **Generate candidates, then decide:** Jev is strongest when selection cannot leave a known set.
- **Structural validation plus semantic verification:** schemas catch shape; Jev checks whether content is supported and on target.
- **Cheap gate before expensive cognition:** routine cases avoid a reasoning model while ambiguous/error-prone ones escalate.
- **Sparse failures require max-style aggregation:** one material flaw should not disappear into an average quality score.

## Gotchas and failures

- Pre-parsed extraction cannot return a correct value absent from the candidate set. Over-finding improves recall but increases Choice competition and hits the 255-option limit.
- Semantic attributes can still be wrong even when the selected span is verbatim; downstream normalization must validate parseability and locale assumptions. The money example explicitly warns that its decimal parsing assumes US punctuation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:270-276`
- The SDE walkthrough uses one hard-coded failure and an internal 100-prompt chart. It does not publish labels, exact frontier coordinates, verifier precision/recall, or reproducible aggregate tables.
- “Bad = true” improves consistent gating but does not establish calibration; thresholds still require labeled representative data.
- An independent verifier can share systematic blind spots with the extractor or be influenced by adversarial source text.

## Task recipes

### Verify an agent claim before accepting completion

1. Treat the agent's claimed result as the cheap extraction and collect the exact tool outputs/test evidence as source.
2. Keep deterministic checks—exit codes, file existence, JSON schema, counts—in Gimble code.
3. Ask per-claim Nouls for unsupported, off-target, incomplete, and contradicted evidence, framing “something is wrong” as true.
4. Escalate if any material check crosses its calibrated threshold; do not average it with unrelated passing checks. Follow `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/sde_cascade.md:636-658`.
5. Send only flagged claims and evidence to an expensive reviewer.

### Select an exact evidence token or artifact

1. Generate candidate IDs deterministically from run events, file paths, test names, or parsed citations.
2. Give Jev a Choice over IDs plus `none`; never ask it to retype an identifier.
3. Copy the chosen value from the candidate map and validate it in code before use.
4. If there are more than 255 candidates, retrieve a section/window first and then choose within it. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/pre_parsed_value_extraction_cookbook.md:306-314`.

## Gaps

- No reproducible aggregate SDE table or public verifier confusion matrix.
- No measurements for agent claims, completion checks, or tool-result verification.
- No guidance for correlated verifier failures or multiple-comparison effects under very large batteries.

