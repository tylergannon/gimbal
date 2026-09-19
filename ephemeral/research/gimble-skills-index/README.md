# Gimble skill research index

This index supports Tyler's editorial review of three future sub-skills:
**Author workflows**, **Build and release Gimble**, and **Use Gimble**.
It gathers the teachings behind the commands, including workflow structure,
development priorities, proof, promises, and chapter/sprint vocabulary.
It is supporting research for the three skills linked above.

## Start from the task

| Your question | Read next |
| --- | --- |
| What should the three skills teach, and in what order? | [Skill contents](../../../skills/gimble/SKILL.md) |
| How should I structure a workflow, its agents, context, and feedback? | [Authoring route](routes/authoring/index.md) |
| What priorities govern building, validating, merging, and installing Gimble? | [Build and release route](routes/release/index.md) |
| How do I call a workflow, steer it, and decide whether its promise was fulfilled? | [Usage route](routes/use/index.md) |
| Which evidence wins when old guidance and current behavior differ? | [Authority and conflicts](routes/authority/index.md) |

Choose one route, then one relevant leaf, then open its source citations.
Do not read every leaf to answer one question. A typical answer takes four
file reads: this entrypoint, a route, a leaf, and the cited source.

## Token cache and scope

Primary local token cache: `/Users/tyler/.codex/worktrees/365e/gimble`.
It is the existing repository, read without modification during indexing.
The selected sources span the current Go implementation and tests, repository
instructions, build configuration, current and historical design records,
`df-*` skills, and selected worklogs. Frozen `ephemeral/legacy/` material is
used for teachings and rationale, never as the current API.

The supplemental source family is six explicitly selected instruction files
under `/Users/tyler/.agents/skills/`. Those citations use absolute paths;
all others resolve relative to the primary token cache. The exact source list,
assignment boundaries, revision and SHA-256 fingerprints are in the
[source manifest](.semantic-index/manifest.json). This is a selected research
corpus, not an index of every file or dependency in the checkout.

Index root: `/Users/tyler/.codex/worktrees/365e/gimble/ephemeral/research/gimble-skills-index`.
Sources were already materialized locally; no remote synchronization was needed.
The index itself and generated build/dependency trees are outside the corpus.

## Read citations with their authority

`path/to/file:L10-L25` identifies source lines in the manifest’s recorded
source revision. The skills were rewritten using this research; for their
original cited wording use `git show <source_commit>:<path>` from the checkout. Leaves label claims as current
implementation/instruction, design record, historical, or proposal. Current
user direction and repository instructions govern the task; current code,
Godoc and executable examples establish available behavior. Design records
explain decisions but may use replaced API names. Generic skills are adaptable
methodology; they do not silently override Gimble-specific instructions.

An existing command is different from a proposed built-in workflow. A
`PromiseLoop` planner is different from a repository-level promise contract.
A fulfilled promise is different from a completed run. The routes preserve
these distinctions rather than flattening the source families into one manual.
