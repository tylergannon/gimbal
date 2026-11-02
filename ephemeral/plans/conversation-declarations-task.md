# Implement conversation environment declarations and pure preflight

This is an authorized coding assignment, not another planning pass. Work in
`/Users/tyler/.codex/worktrees/1430/gimbal`. Implement the complete outcome below
and run affected checks; do not stop after unrelated easy fixes. Parent contract:
`ephemeral/plans/distributed-execution-slice.md`; prerequisite:
`ephemeral/plans/conversation-placement-prerequisite.md`.

Outcome: an ordinary registered workflow can decorate a context using
`gimbal.WithEnvironment(ctx, "literal-name")` or
`gimbal.WithEnvironment(ctx, params.Environment)` where Environment is a declared
direct string input. Generated graph/catalog metadata carries literal names and
input-field references. Pure registered preparation resolves the selected names
from validated input and rejects missing, empty, undeclared/computed or unconfigured
selections before workflow effects. Scope inheritance and session creation capture
the selected logical name; a later call context cannot move an admitted session.

Implement this through the existing context declaration extractor, workflow
metadata, Registry/preparation and runtime context path. Add the environment input
to the registered Conversation workflow and pass the UI/manager selection into its
validated input at host launch. Physical Route.Environment must not remain its
placement authority. Integrate schema/generation changes through generators.

Do not create a standalone proof program, generic placement framework, new workspace
manager or speculative exported functions. WithEnvironment is the approved authoring
addition. Existing unrelated routes can remain outside the new composition, but an
invalid explicit name cannot execute locally. Preserve #665 and the preceding
clock-skew/completion-ID changes in the working tree.

Add meaningful tests for literal/input declaration extraction, rejection before
effects, inheritance and captured session placement. Run affected packages, generation
and compile checks. Native namespace delivery will be the next assignment; do not
claim live C1 from these checks. Do not alter shared services, start remote native
workers, commit, merge or publish. Report exact source changes and verification.
If a genuinely underivable seam decision prevents this outcome, report that specific
decision rather than returning a generic list of unfinished parent requirements.
