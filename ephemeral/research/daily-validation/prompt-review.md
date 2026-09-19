# Research-document prompts and semantic-index translation review

Reviewed 2026-09-19. Review only; no workflow, prompt, skill, or helper changes.

The translation retains useful parallel topic assignments, local files, citations, and a single author entrypoint. It loses the skill's central optimization: help a future agent reach original evidence with little reading. The workflow instead builds successive summaries. These runs are not a controlled evaluation of Flash versus Pro or of the semantic-index skill in isolation.

Scope: current research-document prompt constants, their recorded rendered prompts and tool activity in the two completed research runs, the supplied df-semantic-index skill, its model reference and evaluation helper, resulting indexes, and the task briefs. Model resolution for this audit returned main=inherit and reader=inherit. No new model runs were launched. Relevant review guidance: evaluate-skills and write-prompts.

## Findings, ordered by impact

### 1. Bug: the translation changes retrieval into synthesis

`internal/workflows/researchdocument/researchdocument.go:455-465` asks researchers for a "goal-relevant synthesis" and the curator to "preserve the important knowledge." The skill instead defines a compact routing tree over a distinct token cache (`.agents/skills/df-semantic-index/SKILL.md:9-22`), with exact citation bookmarks in leaves and retrieval-oriented integration (`:161-192`).

The workflow retains directories and local links but does not state what belongs in routing nodes versus evidence leaves. Its author prompt permits following links to longer clips "only when needed" (`researchdocument.go:467`). This makes repeated summaries an easy stopping point. The prior-art root index measures 13,213 o200k_base tokens, over twice the final document's 6,500-token budget. The feature root is 10,286 tokens. These are measurements of current local index files, not historical input-token usage.

Small repair: define the index as short task-oriented routes to specific evidence, with annotations only sufficient to choose the next source. Preserve longer explanations in source annotations rather than repeating them at every level. Allow the author to inspect linked original sources as well as clips.

### 2. Bug: source preservation and source interpretation are not reliably separated

The research prompt says "download" useful sources, which is directionally correct. However, its output contract does not distinguish original retrieved text from an agent's explanation of it. Some files labeled "Primary Source" in this run are generated capability summaries. The skill's separate token cache/index distinction would make that boundary clearer; research-document combines collection and indexing under the same topic directory without defining their semantic separation.

The actual brief explicitly requested supported facts and original URLs, yet misleading summaries still passed. `verifyResearchFloor` (`researchdocument.go:419-438`) counts directory entries and requires a nonempty INDEX.md; it does not even require individual source files to be nonempty, and it does not evaluate the returned unresolved questions. Thus the enforced floor is a file count, not evidence quality.

Small repair: preserve retrieved source text or faithful excerpts with origin/revision and section/line pointers; put researcher interpretation in separate annotations. Check file existence/nonemptiness and citation resolution mechanically, while leaving source sufficiency to a reviewer.

### 3. Bug: the independent editor shares the same evidentiary weakness

`editorialPrompt` (`researchdocument.go:469`) explicitly requests reading the document and semantic index, not verifying consequential claims against original sources. It can catch inconsistencies and poor prioritization while agreeing with factual errors repeated in both inputs.

In the completed prior-art run, the editor read the brief, our correction index, the final document, and all five topic indexes. It listed two checked original README files with `ls -l`; its recorded tool calls do not open those originals or the topic source files. The feature editor did inspect repository source for some claims, so this is a weak default contract, not a universal inability to verify.

Small repair: ask the editor to trace decision-relevant claims to original passages and identify unsupported claims or unresolved contradictions as material issues. Keep factual verification distinct from document compression.

### 4. Bug in the original skill: the benchmark does not test its stated quality bar

The skill promises retrieval quality and a hill-climbing improvement loop (`SKILL.md:238-294,406-414`). Its helper performs query-independent breadth-first traversal; query terms score visited files but do not choose routes (`scripts/run_evals.py:97-113`). More seriously, expected-file existence awards a hit even if the file was never reached, and an acceptable prefix can award a hit for a nonexistent file (`:153-168`). "Citation precision" divides matched expected files by expected files; it does not penalize irrelevant retrieval. "success@1" is not success on a first retrieval decision.

A temporary isolated probe with an entrypoint containing no links received success_at_1=true and citation_precision=1.0 for an existing unlinked file, and again for a nonexistent file with an acceptable prefix. Each probe read only the entrypoint. No probe program or output is committed.

The recorded prior-art curator actually opened the full df-semantic-index skill and ran its helpers. Its saved benchmark reports 100% success but a 26.55 tool-call estimate against the skill's target of at most five. The workflow itself gates only the root file's nonemptiness (`researchdocument.go:318`).

Small repair: treat deterministic graph checks as structural checks, fix false positives, and evaluate actual retrieval with held-out questions and an answer/source rubric. Even corrected BFS alone will not demonstrate an agent's retrieval efficiency.

### 5. Bug: generated research questions can change the user's constraints

The recorded prior-art plan asks whether terminal recordings should be actual video or event streams, despite actual video being a requirement. It asks which Go PTY libraries fit Gimble, steering toward custom implementation, and asks for assertions "without relying on flaky LLM-as-a-judge scoring," importing a conclusion into the question. The complete research plan is appended to the curator, author, and editor prompts, so those assumptions recur downstream.

This is observed planner behavior, not wording literally present in the static planner prompt. `planTopicsPrompt` (`researchdocument.go:453`) requests specific questions but gives no explicit requirement to preserve settled constraints and keep open questions neutral. Our recovery brief also required reusing earlier research, so this run is not a clean fresh-corpus experiment.

Small repair: ask the planner to distinguish fixed requirements from open questions and avoid embedding candidate solutions or unverified conclusions in the questions. Do not add another planning subsystem.

## What is good in the skill, and what to simplify

Keep the separate token-cache/index concept, preassigned nonoverlapping output paths, exact citation anchors, routing by retrieval intent, explicit unresolved areas, and outcome-based quality bar. Those are useful instructions.

The 414-line skill mixes its central retrieval contract with model configuration, script discovery/fallback generation, maintenance, rebalancing, and sprint integration. Those operational details can live in references. The parallel-reader trigger also varies between 10 or more, greater than 10, and greater than 20 files (`:105-112,214`); pick one rule. Neither issue is as consequential as the evaluator defect or the workflow's synthesis framing.

Not every omission is a translation bug: a one-document workflow need not implement all of the skill's long-lived index maintenance and sprint integration. It does need the evidence boundary and a meaningful retrieval check.

## Suggested replacement direction, not applied

Researcher: "Collect original local evidence answering the assigned questions. Preserve source excerpts with origin and precise locations; keep your interpretation separate. Write short annotations linking each supported answer to that evidence, and mark unresolved questions."

Curator: "Build a compact route from likely author questions to the relevant evidence. Each route should explain when to follow it and link to precise source passages or annotated leaves. Keep factual detail in the leaves and sources. Carry forward contradictions and gaps."

Editor: "Assess the document against the brief. Follow citations for decision-relevant claims to original evidence. Report unsupported claims, contradictions, missing requirements, and material prioritization problems."

The next experiment should hold the corpus and retrieval questions fixed, compare current versus revised index prompts on Flash, and judge answer correctness, source support, tokens read, and actual tool calls. Only then vary the indexer model; compare Flash collection plus Pro authorship separately. This isolates prompt effects from model effects. No before/after model-quality improvement has been demonstrated by this review.
