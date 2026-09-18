# Scoped context planning

doc_bug: ephemeral/research/api/API.md describes public Get/GetJSON/ScopeText and manual prompt assembly; current Godoc and session.go instead expose automatic injection plus WithScopeTemplate. Use current API when planning budgets.
decision: Context visibility, artifact lifetime, and resident memory are separate requirements. The requested regression proves parent/sibling injected context after child closure; it cannot erase prior turns in a native session.
decision: RunCommand promises complete return strings today. File streaming alone cannot establish bounded end-to-end memory while preserving that promise. The plan leaves the proposed bounded-return semantics for discussion.
correction: Tyler settled command returns: stream to files, read bounded head/tail excerpts, and include paths in the returned text. Agents can follow the paths; do not turn this into caller migration or a contract approval gate.
decision: Tyler wants practical token accounting with roughly 50 percent tolerance and suggested tiktoken-go/tokenizer. The revised proposal uses its o200k_base Count method across harnesses, without model-specific token matching.
correction: A run is not relocatable or resumable after moving it. Relative structured artifact references and absolute agent-facing paths apply only to the run in its original directory; do not build portability machinery or tests.
decision: A failed artifact spill from Set or SetJSON panics immediately, as a failed value encoding does today. Do not add sticky run-error state or change the public Set API.
decision: Focus validation on approximately seven Go test functions across the branch, including the existing scope-isolation regression, plus one gpt-5.6-luna run proving an agent can recover omitted content through the supplied file path. No browser proof or committed proof artifacts.
