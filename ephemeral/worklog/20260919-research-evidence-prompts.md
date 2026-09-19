# Research evidence prompt rewrite

decision: User requested a prompt-only translation of df-semantic-index within research-document's existing workflow shape, followed by PR/merge/install. Measured retrieval evaluation is a separate GitHub issue; no schema, stage, model default, or runtime-gate changes belong in this patch.
correction: Current main already contains #299 with Flash collection/indexing and Pro author/editor defaults. Preserve these defaults; a bounded live validation uses Flash Medium for every role under the repository's cheap-model proof policy.
skill_issue: Research-document treated index nodes as successive summaries and did not explicitly require original-source verification by author/editor. Prompts now preserve original evidence separately from interpretation, route by retrieval intent, retain settled requirements and uncertainty, and check consequential claims against original passages.
