# Adversarial review: promises-ledger plan, round 03

> Recovered from the session log. The original `ephemeral/reviews/202610130800-promises-ledger-plan-round-03.md` was git-excluded and was deleted with the worktree. Only the Findings and Outcome sections survived.

## Findings

### 1. Critical — `--tools ""` still permits repository settings hooks to execute the transcript prompt

The plan treats `claude.WithoutTools()` as the complete safety boundary for
untrusted speech: it runs Claude in `<project>/.gimbal/promises/work`, passes
`--tools ""`, and concludes that this leaves the model no way to act on the
machine (`plan.md:112-115,254-263,434`). That only removes model-invoked tools.
The current adapter separately and deliberately loads the repository's
`project,local` Claude settings (`claude/claude.go:70-95,164-167`) while running
with bypass permissions (`claude/claude.go:144-150`).

Claude settings hooks are executable commands independent of the model's tool
list. The pinned SDK's integration test documents that a `settings.json`
`UserPromptSubmit` command hook is run by the CLI
(`/Users/tyler/go/pkg/mod/github.com/tylergannon/claude-agent-sdk-go@v1.2.1/integration_test.go:1960-1991`),
and the hook contract gives that hook the submitted prompt
(`/Users/tyler/go/pkg/mod/github.com/tylergannon/claude-agent-sdk-go@v1.2.1/docs/examples/hooks.md:205-224`).
Because the ledger prompt contains the transcript and the cwd is inside the
admitted repository, a project's `.claude/settings*.json` can therefore receive
every requirements batch and execute a command each time even when
`--tools ""` is present.

Impact: merely speaking into the background ledger can execute repository hook
commands with Gimbal's privileges and disclose the transcript to them. This is
the same unattended side-effect class the tool denial is supposed to remove.
The ledger-specific adapter must also exclude project/local settings and hooks
(without breaking CLI authentication), and its test must establish that a
settings hook does not fire, not only that the tools flag was passed.

### 2. Issue — the required existing-recording action is still unavailable for recordings beyond the newest 25

The authoritative request requires a button on an existing recording, and the
plan's own done demo requires opening an older ordinary recording
(`plan.md:12-15,45-46`). `DurableStore.List` exposes pagination but returns only
25 records at a time (`internal/transcript/durable.go:239-269`). The workspace
still requests only `after: ''` and renders only that result
(`web/src/lib/workspace/ProjectWorkspace.svelte:24-31,149-150`).

The revised plan pages all recordings in the background worker
(`plan.md:181-186`), but explicitly declines to add any UI pagination or other
discovery path (`plan.md:436`). Putting the mark button inside a recording that
the user cannot reach does not satisfy the action. With 26 recordings, the
oldest record has no link in the normal UI; without already knowing its opaque
recording id, the user cannot open it to press “Treat as requirements
discussion.” Worker pagination cannot repair that reachability failure.

Impact: the feature stops applying to older existing recordings as a project
accumulates history. The plan needs a load-more/pagination/search affordance or
another normal UI path to mark an older recording, with browser proof using
more than 25 records.

### 3. Issue — progress on one recording erases the only visible failure for another recording that remains unprocessed

Round 02 correctly changed scheduling to per-recording backoff, so recording B
can progress while recording A remains failed (`plan.md:181-183,236-239`). The
status model did not become per-recording: `Store` has one `err` string
(`plan.md:68-75`), and every successful batch clears it unconditionally
(`plan.md:200-204,206-218`).

The direct reproduction is the plan's own new scheduling test shape: A returns
invalid output and enters backoff; on the next step B succeeds. B's success sets
the sole error to `""` even though A's cursor is unchanged and its retry is
still pending. The Promises tab then reports no error for the requirements
recording whose speech has not accumulated. The proposed test only asserts that
B progresses; it does not assert that A's unresolved failure stays visible
(`plan.md:402-405`). This also conflicts with the research integration boundary
that a failed LLM batch leave its cursor unadvanced **and surface the error**.

Impact: the new non-starving scheduler can make a persistent ingestion failure
disappear specifically because unrelated work succeeded, leaving the user no
reliable indication that one recording was skipped. Failure state must retain
recording identity (or remain aggregated) until that recording succeeds or is
otherwise resolved, and the persistent-A/successful-B test must cover the
visible status.

## Outcome

**material findings remain**
