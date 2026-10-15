# Session/workspace architecture review

## Target and verdict

Reviewed `9d9cb3a470f8178a2c8b6ecb3fe978eb87eba290` against `1e387b342bc88ce29d325f422d8314c43fb79009`, with `ephemeral/issue-574.md` as the authoritative requirement. The specific question is whether this is a valid, idiomatic, simple approach worth finishing, or a fundamentally wrong or convoluted design.

**Retain the architecture.** The ownership and data flow are sensible and proportional to the feature. I found no architectural defect requiring replacement, a new persistence mechanism, a new native bridge, a state-machine framework, or re-engineering the composer. The implementation is mostly idiomatic, but I would not call every detail idiomatic: the native Start Like request uses the counter-prop/effect pattern explicitly discouraged in this repository. There are also local correctness defects. These distinguish “sound approach with bounded repairs” from “finished and proven.”

The prior two reported bugs do not imply a foundational failure. I independently inspected their causes rather than treating the earlier report as architectural authority. One is an empty-list assumption inside a picker; the other is one status-projection precedence decision. Neither invalidates the chosen boundaries.

## Evidence inspected

- Repository `AGENTS.md`, agent protocol, adversarial-review and Svelte-component-factoring instructions; `skills/gimbal/SKILL.md`, `docs/definition-of-done.md`, and `ephemeral/research/svelte-idioms/dos-and-donts.md`.
- The full relevant implementation diff, including generated transport changes, and surrounding conversation creation/resume, environment observation, host window storage, retained panels, route layout, native capability/command bridge, and the affected tests.
- Existing model alias/catalog resolution and adapter effort handling in `claude/claude.go`, `codex/codex.go`, and `agy/agy.go`; model-catalog refresh guidance.
- Official Svelte documentation for [derived state](https://svelte.dev/docs/svelte/$derived), [effects](https://svelte.dev/docs/svelte/$effect), [reactive collections](https://svelte.dev/docs/svelte/svelte-reactivity), and [remote functions](https://svelte.dev/docs/kit/remote-functions). Official [Claude effort documentation](https://platform.claude.com/docs/en/build-with-claude/effort) supports the currently catalogued Claude thresholds/levels represented here. This was a narrow check, not a live certification of every provider/model combination.

Checks run independently during this review:

- `go test ./internal/modelalias ./internal/conversation ./internal/host`: passed; Go reported cached results.
- `pnpm exec vp test --run src/lib/conversation/NewSession.svelte.test.ts src/lib/conversation/ConversationView.svelte.test.ts src/lib/workspace/conversations.test.ts src/lib/shell/WorkspaceShell.svelte.test.ts`: exit 1, 22 tests passed, one failed assertion, two unhandled `shown.models` errors from the empty-inventory case. The assertion expected `/Claude Code/` while the reported actual text was `C Claude Code · Haiku 4.5`; that assertion result does not establish a product failure. The unhandled TypeError does.

No implementation files were changed and no native acceptance exercise was performed. The ordering risk below is source-backed, not a newly reproduced browser failure.

## Why the architecture is appropriate

### Draft choices have the right owner

`NewSession.svelte:26-58,80-89` owns its form, selected environment, per-environment choices, and recent-setup interpretation. `SvelteMap<string, Choices>` is a small, supported reactive collection. The message remains in the real form while picker choices change; no form reconstruction, global draft controller, or duplicated transcript store is introduced. Per-environment selection retention directly serves the issue. Capturing defaults once is also the right intent: deriving a live draft selection from a mutable recent-session ordering would let background activity change the person's draft. The capture timing needs the local correction in finding 3.

### Durable facts stay with the domain that owns them

`Conversation` gains `Effort`, `SharedCheckout`, and `Base`, and all three creation paths retain them (`internal/conversation/manager.go:321-325`, `remote.go:91,871`). This is straightforward metadata for reproducing a setup; it is preferable to reverse-engineering intent from the resulting directory or branch name. Rename changes the existing durable title and revision while preserving session identity (`manager.go:758-779`).

`internal/host/window.go:17-28,58-107` extends the already existing `window.json` owner with an optional sidebar preference and completed-request acknowledgments. Its mutex, project-scoped merge, and atomic replacement fit the existing single-window design. The layout's small in-memory merge and ordered save chain (`+layout.svelte:17-23`) address project remounts and write ordering; the host merge protects persisted facts. They have different lifetimes, so the two merges are not evidence of gratuitous infrastructure.

The read marker records a conversation path plus completed request identity, not transcript copies or guessed timestamp ordering. `completedReply` excludes steer acknowledgments (`workspace/conversations.ts:26-35`). This is a sensible minimal identity scheme for the requested unread behavior.

### Native state remains a capability projection

`ProjectWorkspace.svelte:180-188` sends active-view capabilities and menu labels through the existing bridge. Swift stores that presentation and dispatches commands (`WorkspaceMenu.swift:6-52,70-142`); it does not gain independent conversation execution logic or a second session model. Active chat views consume Rename/Stop commands only when active (`ConversationView.svelte:69-77`). That is the appropriate boundary, including keeping workflow-agent controls out of top-level chat commands.

Retaining keyed panels is existing workspace behavior (`WorkspaceShell.svelte:93-98`). Giving the composer one picker discriminator and unmounting inactive picker content, while preserving the form, is a reasonable small extension (`NewSession.svelte:44-45,120-154`). The rename component owns its real form and remote mutation, consistent with repository factoring rules.

### Effort uses existing knowledge and adapter paths

`internal/modelalias/effort.go` derives eligible model identities from the existing alias families rather than introducing a UI-side model list. Its small support policy is used for both picker options and server admission (`internal/conversation/effort.go:9-37`). Unknown capabilities remain Default-only. Empty effort flows to the existing adapters, which omit an explicit effort setting; explicit effort is stored on the conversation and reused for native creation/resume (`requests.go:156,184,191`; remote session creation at `remote.go:158-159`). This is a normal, bounded extension of the existing design.

The hand-maintained effort thresholds do need to stay aligned with provider support as the generated model catalog grows. That is a maintenance obligation, not evidence that a generalized provider-policy subsystem is necessary. No new transport, scheduler, retry framework, or orchestration abstraction has been added.

The 42-file diff overstates the amount of architecture introduced: it includes generated serializers/types/remotes, regression tests, and tracked issue/review/worklog notes. The generated bulk is the consequence of extending existing typed remotes, not dozens of new independently maintained layers.

## Findings

### 1. Issue: retained setup can violate the picker's nonempty-inventory assumption

Locations: `web/src/lib/conversation/NewSession.svelte:58,82-88,122`; `HarnessPicker.svelte:16-18,41-45`.

A saved setup keeps `choice` populated even if the selected machine has no harness rows. The parent mounts the picker based on `setup && choice`. `shown` then becomes undefined and the picker dereferences `shown.models`. The existing saved-setup/no-harness regression independently produced this TypeError in this review.

This violates setup reuse and recoverability. Correct the picker contract/rendering for an empty inventory while preserving the selected setup and environment controls. This is a local empty-state fix, not a reason to change draft ownership or replace the picker design.

### 2. Issue: workspace projection hides attention behind Working

Location: `web/src/lib/workspace/ProjectWorkspace.svelte:71-74`.

The projection puts `status === 'unresolved'` in the Working branch before considering `item.error`. A recovered chat with unresolved native outcome and a restart/recovery error therefore appears Working. This contradicts the issue's requirement that working and attention remain distinguishable. It predates this branch, but the changed projection still carries it.

Correct the projection's status precedence/meaning. The authoritative conversation state already contains the needed facts; this does not call for a new state-management design.

### 3. Issue: delayed recent defaults can overwrite an explicit initial selection

Locations: `web/src/lib/conversation/NewSession.svelte:30-32,49,61-78`; test mock at `NewSession.svelte.test.ts:152-157`.

Source-backed ordering: the setup query and recent-conversations query are independent, and controls are enabled when setup is available. A person can select a harness or checkout through `choose()` while `conversationsQuery.ready` is still false. `choose()` updates the choices map but leaves `initialized` false. When recent conversations first become ready, lines 67-70 call `like(recent[0])`, replacing those explicit choices (and potentially switching environment). `pickEnvironment()` already marks initialization, so environment selection avoids this particular overwrite; harness/checkout selection does not. The current test mock always returns `ready: true`, so the existing “background reply cannot replace setup” test does not cover this ordering.

This is a bounded initialization bug: treat an explicit initial selection as owning the draft before applying late defaults. I did not reproduce the delayed-query sequence in a browser during this architecture review. It does not undermine the choice of draft-local state; it demonstrates why the default-capture boundary needs to be explicit.

### 4. Nitpick: Start Like requests use a counter prop to drive an effect

Locations: `web/src/lib/workspace/ProjectWorkspace.svelte:107-108,162,219`; `web/src/lib/conversation/NewSession.svelte:22-24,31,63-66`; repository idiom guidance §24 at `ephemeral/research/svelte-idioms/dos-and-donts.md:295-301`.

The parent increments `setupNonce`, passes it in `startLike`, and the child compares `appliedSetup` in an effect before executing `like()`. This is the exact counter-prop request pattern the repository asks to replace with a narrow imperative component method. It adds unnecessary coordination state for an ordinary “apply this setup” action.

Use a direct component operation for an existing retained composer, with at most a pending setup value if navigation must create the composer first. This is a local idiom correction, not a controller layer or broad component rewrite. I found no concrete runtime failure caused by the nonce alone, so this is not a foundational finding.

## Disposition

**Outcome: material findings remain.** Retain and finish this implementation. Repair the local empty-state/default-initialization/status defects, simplify the Start Like handoff, and complete the separately required native acceptance. There is no evidence here for calling the work untenable or requiring complete re-engineering, and there is also insufficient evidence to claim the final native behavior is already proven.
