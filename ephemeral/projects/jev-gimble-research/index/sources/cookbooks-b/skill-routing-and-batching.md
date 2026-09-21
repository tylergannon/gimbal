# Skill routing, progressive disclosure, and parallel-question economics

## Purpose

This leaf records the strongest directly agent-related cookbook result: Jev suggests at most one skill from a large roster using wide ranking plus detailed reranking. It also captures the measured economics and stability of batching many independent questions over shared state.

## Key concepts and measured results

- **Skill suggestion uses progressive disclosure rather than loading everything.** Request one ranks all 182 truncated skill descriptions and decides whether any skill is needed; request two reads the top three with full descriptions and instruction excerpts and may reject all. The resulting name is a non-binding hint added after the stable roster. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:5-40`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:57-78`
- **The wide pass separates relative ranking from “should route at all.”** One Choice ranks 182 names, while three Nouls ask whether the request needs action on user systems, documented procedures, or can be satisfied in prose. Their oriented mean is a gate; below `0.30`, no suggestion is emitted. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:435-517`
- **The detailed pass also separates relative and absolute fit.** A Choice chooses among three candidates using richer criteria, while one Noul per candidate asks whether it actually does the requested thing. If every absolute fit is below `0.30`, the shortlist is rejected. Choice and Nouls can disagree because one answers “which?” and the others “whether?”. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:567-680`
- **The system remains fallible when no roster item fits.** A Mastodon request survived both stages and received an X/Twitter suggestion because all three finalists were near-misses and the best fit remained above threshold. The second pass can only reject or select candidates it receives. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:551-565`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:660-677`
- **The evaluation used 488 single-turn requests.** Of these, 315 were synthetically generated from exactly one skill's instructions and 173 were designed to have no matching skill. Scoring only inspected the agent's first response: wrong first skill (or none) on covered requests, and any load on uncovered requests. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:248-267`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:269-287`
- **Suggestions more than halved both measured errors.** With Claude Haiku and a 182-skill Hermes roster, wrong loads fell `16.8% → 7.3%` and needless loads `9.8% → 4.0%`; an oracle hint still had `2.5%` and `1.2%`, showing that even the right suggestion is not always followed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:38-55`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:732-795`
- **Suggestions can harm already-correct behavior.** Among 315 covered requests, the TypeSafe hint fixed 37 baseline misses but broke 7 requests the agent had previously handled correctly. A confidently wrong hint is persuasive; the cookbook therefore makes it explicitly ignorable. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:797-820`
- **The skill-routing result is version- and setup-specific.** It used `jev-1.12`, `claude-haiku-4-5-20251001`, cached calls, and was rendered 2026-07-31. The positive prompts were generated from their gold skill instructions and described as easier than natural user requests. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:95-120`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:248-259`
- **Batching shared-state questions was tested directly.** Thirteen questions (8 Noul, 2 Choice, 3 Score) over a 53,777-character GDPR article were asked as one batch and separately, five repeats per strategy. Most values were identical; two noisy Nouls had similar means and variance under both strategies. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:5-27`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:186-288`
- **The batched example was 12.2× cheaper and 10.0× faster than sequential singles.** One call cost `$0.000497` and took `0.27s`; 13 sequential calls cost `$0.006090` and totaled `2.71s`. The docs explicitly note that concurrent singles reduce the time gap but not the repeated-state token cost. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:290-330`

## Important citation bookmarks

- Two-stage skill-routing architecture: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:5-78`
- Dataset and scoring definitions: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:248-287`
- Wide ranking/gating: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:435-517`
- Detailed rerank/rejection: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:567-696`
- Agent outcome metrics and regressions: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:732-820`
- Batch equivalence experiment: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:186-288`
- Batch cost/latency results: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:290-330`

## Themes

- **Progressive disclosure:** use short metadata for wide recall, then spend context on a tiny shortlist.
- **Absolute gates around relative rankers:** “best available” is not the same as “good enough.”
- **Advisory steering:** a suggestion should remain ignorable because confidently wrong steering can degrade a correct run.
- **Batch the supervision vector:** when many checks share one trace window, send the state once and evaluate the whole atomic battery.
- **Measure agent outcomes, not only Jev labels:** the skill cookbook observes downstream tool use, revealing both fixes and newly broken turns.

## Gotchas and failures

- The 315 positive requests were generated from gold skills, so reported routing accuracy likely overstates performance on ambiguous natural traffic. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:248-259`
- The evaluation is one run per arm/request and grades only the first response. It does not measure recovery, multi-turn completion, task quality, or total cost.
- Mean gating across three Nouls can average one strong “no skill” signal with two weak positives. This is application logic, not a documented optimal aggregator.
- A shortlist made only of near-misses can pass the absolute fit threshold, as the Mastodon example shows. Include a strong `none` path and calibrate on adversarial near-neighbor negatives. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:660-677`
- Batch stability was tested on one large document, 13 questions, and five repeats. It does not establish unchanged behavior for every question count, state length, or model release.
- The 10× speed claim compares against sequential calls; a parallel caller should use token savings as the more robust expectation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:290-330`

## Task recipes

### Suggest a Gimble coaching tactic without taking over control

1. Build a wide Choice over short tactic descriptions and independent Nouls asking whether any coaching is needed.
2. Rerank the top few with full examples, exclusions, and prerequisites; add one absolute-fit Noul per tactic.
3. Emit at most one advisory hint and explicitly permit the agent/supervisor to ignore it.
4. Record whether the hint fixed, broke, or did not change the run. Use the downstream comparison pattern at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/skill_suggestion.md:732-820`.
5. Include difficult no-match and near-neighbor examples, not only positives generated from the tactic text.

### Evaluate a full supervision rubric economically

1. Construct one relevant trace state.
2. Put every independent failure detector, severity dimension, routing choice, and evidence-existence check into one request.
3. Compare a sample of batched versus single calls across repeats before relying on question independence in the target version.
4. Track input tokens and wall-clock time separately; concurrent singles can match latency but still resend the state. Start from `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/parallel_questions.md:290-330`.

## Gaps

- No evaluation on Gimble's skill roster, trace vocabulary, models, or multi-turn outcomes.
- No total cost/latency table for the two-stage 488-request skill experiment.
- No confidence intervals, repeated-agent variance, or analysis of failures by skill frequency/category.
- No proof that batching remains invariant for much larger batteries or near context limits.

