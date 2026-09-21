# Retrieval, evidence location, re-ranking, and hierarchical decisions

## Purpose

This leaf captures how Jev narrows a large search space: direct line Choice plus an existence check, pairwise Noul re-ranking after fast retrieval, and beam search through a hierarchy. These patterns are relevant to locating evidence in run histories and selecting a failure/coaching taxonomy.

## Key concepts and measured results

- **Line search uses stable IDs as Choice options.** The semantic-find recipe tags 218 clauses, asks one Choice to distribute probability over line IDs, and uses the probabilities as a ranking. The state remains the tagged document while the query changes in instructions. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:5-31`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:74-141`
- **Ranking must be paired with an absolute existence check.** Choice probabilities sum to one and therefore rank something first even when no line answers. A Noul in the same request asks whether any answer exists, allowing found/partial/absent routing independently of the ranked line. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:143-209`
- **The four semantic-find examples demonstrate the forced-winner failure.** Two direct answers had existence `0.98` and `0.97`; an arbitration query's closest line scored `0.86` but existence was `0.14`, and parental permission ranked a line at `0.90` while existence `0.46` classified it as only partially addressed. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:211-242`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:244-300`
- **Choice's 255-option ceiling makes direct search local.** Larger documents require a two-pass window-then-line search rather than one flat Choice. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:121-141`
- **Re-ranking is explicitly a second stage.** Fast keyword or embedding search retrieves a shortlist; Jev scores each query-candidate pair with the same Noul and code sorts descending. It cannot recover an item missing from the first-stage shortlist. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:9-22`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:34-77`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:93-144`
- **CLERC re-ranking improved rank metrics but remained imperfect.** On 40 queries over 3,565 court passages, BM25's top-30 contained the correct passage for all queries but put it first only 5%. Jev re-ranking changed top-1 from 5% to 18%, top-5 from 15% to 35%, and top-10 from 38% to 62%. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:179-184`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:373-384`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:535-551`
- **The re-ranking workload was 1,200 independent calls.** Forty queries × 30 candidates used 1,536,002 input and 25,200 output tokens and was reported at `$0.0645`; calls ran through a 12-worker pool. The result is tied to `jev-1.12` and the stated price. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:201-230`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:386-456`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:519-545`
- **Hierarchical classification turns each sibling set into a Choice.** Greedy search follows the local maximum and cannot recover from an early error. Beam search retains `K` paths, evaluates frontiers in parallel, and ranks paths by the geometric mean of edge probabilities to normalize for depth. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:5-22`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:45-68`
- **The hierarchy implementation makes search behavior observable.** It records distributions at every queried node, retained paths, path scores, and top/second separation. The docs recommend log-space path scores for very deep trees to avoid precision loss. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:56-68`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:618-770`
- **Beam width three beat greedy on four labeled examples.** Beam matched 4/4 expected leaves while greedy matched 2/4, recovering the patent and product examples. This is a four-example demonstration across four different hierarchies, not a statistically meaningful benchmark. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:773-860`

## Important citation bookmarks

- Direct Choice search and Noul existence pattern: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:121-209`
- Semantic-find examples and failure interpretation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:244-300`
- Re-ranking architecture: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:34-144`
- CLERC measured results: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:373-456`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:519-551`
- Beam-search formula and implementation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:45-68`; `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:618-770`
- Hierarchy results: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:849-860`

## Themes

- **Retrieve broadly, judge narrowly:** a cheap first stage protects scale; Jev spends semantic judgment only on a shortlist.
- **Relative ranking needs abstention:** every closed Choice has a winner, so pair it with an absolute fit/existence signal.
- **Delay irreversible commitment:** beam search preserves alternatives when early categories are ambiguous.
- **Record the path:** per-node and per-candidate probabilities reveal where taxonomy or retrieval errors arise.

## Gotchas and failures

- Re-ranking cannot repair first-stage recall. The cookbook's 100% top-30 recall was unusually favorable and should not be assumed for Gimble traces. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:373-384`
- Even after Jev re-ranking, CLERC top-1 was only 18%. This pattern is better for candidate ordering and escalation than unilateral evidence selection. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/rerank_typesafe.md:535-551`
- The semantic-find thresholds (`0.7` found, `0.35` absent) were chosen to separate four examples and explicitly require tuning on other documents. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:211-242`
- Choice probability over line IDs is relative to the candidate set; changing chunking or adding distractors can change the distribution.
- The beam-search result is only four labeled cases. Beam adds model calls and can preserve several wrong paths if the taxonomy descriptions are poor.

## Task recipes

### Retrieve the trace evidence for a supervision alert

1. Use deterministic search/recency rules to shortlist trace events or windows.
2. Ask one Noul per query-window pair for absolute relevance, then sort by probability.
3. Separately ask whether the available trace contains enough evidence at all; do not force the top-ranked event to count as support. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/semantic_find.md:143-209`.
4. Give the top several events—not just top one—to the reviewer until measured top-1 performance warrants narrowing.

### Walk a coaching/failure taxonomy

1. Define a tree whose leaves are actual interventions or known failure modes.
2. Use a Choice over direct children and retain a beam of plausible paths when the top/second separation is small.
3. Aggregate path probability with a depth-normalized geometric mean; use log space for deep trees. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/cookbooks/hierarchical_classification.md:45-68`.
4. Log every node distribution and adjudicated leaf so weak branches can be rewritten or split.
5. Add a separate absolute “does any leaf fit?” Noul before taking action.

## Gaps

- No retrieval benchmark on agent traces, logs, or tool events.
- No comparison against embeddings/cross-encoders stronger than plain BM25.
- No measured effect of candidate count, chunk size, distractor density, or hierarchical beam width.
- No source-span explanation beyond the chosen/ranked identifier.

