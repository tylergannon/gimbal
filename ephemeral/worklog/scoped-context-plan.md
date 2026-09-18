# Scoped context planning

doc_bug: ephemeral/research/api/API.md describes public Get/GetJSON/ScopeText and manual prompt assembly; current Godoc and session.go instead expose automatic injection plus WithScopeTemplate. Use current API when planning budgets.
decision: Context visibility, artifact lifetime, and resident memory are separate requirements. The requested regression proves parent/sibling injected context after child closure; it cannot erase prior turns in a native session.
decision: RunCommand promises complete return strings today. File streaming alone cannot establish bounded end-to-end memory while preserving that promise. The plan leaves the proposed bounded-return semantics for discussion.
