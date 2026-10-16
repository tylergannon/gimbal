# Independent review: Kit focused-form HMR repair

Outcome: no findings.

## Target and authority

Reviewed the working implementation in this worktree against `ephemeral/issues/601.md`, the user's request to implement the researched direct upstream integration, `AGENTS.md`, `docs/definition-of-done.md`, the Gimbal release skill, and the Svelte idiom rules. The user explicitly assigned #603 to another agent; this review does not certify that issue. No caller instruction was used to suppress a defect within #601.

## Evidence inspected

- The complete maintained dependency patch and lockfile diff, package declarations, existing queue/query hunks, installed Kit form attachment and focusout handler, and Gimbal's actual `NewSession.svelte` composer and Go `createConversation` handler.
- Svelte 5.57.2's public event registration implementation, its delegated event ordering, returned disposer, `without_reactive_context` implementation, and mutation guard.
- The uncommitted disposable browser exercise at `tmp/601/composer.mjs`; independently executed it with `vp exec node ../tmp/601/composer.mjs` from `web/` against the running real Gimbal development server.
- The final zero-context patch representation and refreshed patch hash, plus `git diff --check`.
- The current generated Go changes. They are development-host output, not part of the intended fix; the implementer must complete the already planned canonical regeneration before delivery. This review does not approve shipping those transient generated differences.

## Assessment

The patch imports `on` from public `svelte/events`, uses it for Kit's existing focusout handler, and calls its returned disposer in the existing attachment cleanup. It reuses upstream's actual event machinery rather than copying a private helper or inventing Vite coordination. Svelte clears and restores both reactive context pointers around the handler, including exception paths. Cleanup removes the actual wrapped function. The pre-existing queue/query repairs remain unchanged; lockfile changes only propagate the new patch identity.

The independent browser run showed ordinary focusout updating touched/dirty state, three consecutive focused component replacements retaining the exact multiline draft without a document reload, one listener on the connected composer, and no listener retained on detached forms. Navigation away and back followed by another focused HMR replacement also retained the draft and correct listener counts. Browser error collection remained empty. Setting touched before HMR does not mask the unsafe mutation: Svelte's mutation guard runs before the equality bailout.

After those replacements, an empty-message form submission reached the actual Go handler and returned its expected validation rejection. The handler rejects that input before `manager.Start`, so no model session was started. That establishes the submission transport remains operational without claiming successful model execution or repaired validation feedback. The source heading was restored by the exercise's cleanup.

There is no material proof gap for #601's focused-HMR exception, draft preservation, ordinary focusout bookkeeping, or listener cleanup. Successful model execution, #603's feedback behavior, distributed artifacts, and installed native app behavior are not established by this run and should not be claimed.

## Findings

No material findings or genuine nitpicks.
