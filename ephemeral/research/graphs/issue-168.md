# Sprint workflow: a validator that can't reshape the goal, plain local prompts, and prompts visible before they run

URL: https://github.com/tylergannon/gimble/issues/168
State: closed
Updated: 2026-09-14T00:50:54Z

## Where this comes from

This comes from the attempt to write the workflow for **Sprint 3: the web app, first pass** (`ephemeral/research/api/SPRINTS.md`). That attempt was PR #121, "Work issues beside the sprint, and supervise the validator". Tyler reviewed its prompts, judged it a failure, and closed it unmerged on 2026-09-13. Main had moved too far for the branch to be worth rebasing, so none of its code carries over. This issue records what the review found, so the work can start fresh in a new worktree.

The workflow now lives in `internal/workflows/sprint/sprints.go`. It still has the main problem found in the review; see 1 below.

## What the review found

### 1. The validator can reshape the product

`sprints.go` ~line 107 appends the validator's objections to the goal and runs the loop again, for up to three rounds:

```go
goal = text + ... + "\n\nThe validator found the validation of this goal invalid, so it is also done only when these are answered:\n\n" + objections
```

This lets the validator ask for more proof machinery and more features, and each request becomes a requirement. Tyler: this "has been shown to be extremely toxic to the process." `docs/definition-of-done.md` already says validation "is not a blank check to ask for enhancements". The code has to hold to that.

What the fix needs:
- **The validator's prompt is one line.** It can come with context data on how validation works, such as the Validation section of `docs/definition-of-done.md`. Tyler, on a 15-line draft: "15 lines means you're over-thinking it."
- **The validator's findings never become the goal.** They go to the planner as information: what the validator did not see working. They are not instructions to the builders.
- **The validator's goal has no definition-of-done boilerplate and no sprint "Proof:" line.** Sprint 3's proof, "Sprint 4 is built by `cmd/sprint`…", can't happen inside Sprint 3, so a validator shown that line objects forever.
- **No mechanical filters.** A filter that drops findings quoting nothing in the goal was tried in #121 and rejected as brittle.

### 2. Prompts are plain English, and what they need is already local

#121's issue goal read: `GitHub issue #120, which gh issue view 120 shows, is resolved as it asks, and the pull request that merges it says "Fixes #120".` Tyler called it weird. The prompt should say what it means: "Read and implement issue 120."

Pointing an agent at a remote source is banned. The workflow writes the issue's text into a file first, and the prompt names the file. Tyler: the local filesystem is the store and cache for all information crucial to the task, because that "enhances the agent's ability to find it".

### 3. Every Generate call's prompt and schema can be seen before it runs

Tyler couldn't see what the validator would be sent, because prompts were built from constants, format strings, the goal and `ScopeText`. The response schema is sent too, and its field descriptions are prompt text: the Claude adapter passes it with `--json-schema`. Reviewing a workflow needs a way to see every `Generate` call's actual prompt and schema. In order of preference:

1. **Project an example value** for each prompt, with interpolation and context data filled in. This is exact only where the prompt is built from code, the input and the repo.
2. **Failing that, a decent guess.** A dry run that calls no model: it records each prompt and schema, and answers each turn with a clearly marked example value derived from the schema, so later prompts can be built. It is a viewer, never proof.
3. **Failing that, capture from a real run.** Record the schema in the `turn_started` event beside the prompt, and read the prompts from a cheap-tier run's files.

Workflow reviews should show prompts as values, never as constant names.

### 4. AGENTS.md is wrong on committing

Main's `AGENTS.md` says "Commit only when Tyler asks." That is not Tyler's rule. His standing policy is to commit, and push, every time something interesting has happened. #121 fixed the line and added an "Information lands locally" section (point 2), but both died with the PR. Reread the rest of AGENTS.md against current practice while fixing this.

## Done when

- `AGENTS.md` says to commit and push whenever something interesting has happened, and has the "Information lands locally" rule.
- The sprint workflow's validator:
  - has a one-line prompt, plus context on validation;
  - hands its findings to the planner as information, never as additions to the goal;
  - sees no "Proof:" line or definition-of-done boilerplate.
- Issue text reaches agents as local files, and prompts are plain English.
- There is a way to see each `Generate` call's prompt and schema as values, at the highest tier of 3 that is feasible. The PR shows every prompt in the sprint workflow that way.
- A cheap-tier live run (Codex `gpt-5.6-luna`, Claude `haiku`) shows the validator reporting without adding requirements. Say which models ran.

Related: #120, supervision not yet validated live. It can be folded into the same run.

