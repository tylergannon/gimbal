# Technical handoff for shared workflow-start remotes

Produce the implementation detail needed to execute the accepted plan in
`/Users/tyler/.codex/worktrees/d798/gimble/ephemeral/research/static-workflow-entrypoints/implementation-plan.md`
and its adjacent `definition-of-done.md`. Read both first. The adjacent
`assessment.md` is historical evidence; its separate REST recommendation was
superseded. Do not reopen the agreed architecture.

The implementation audience is the agents working in these isolated checkouts:

- Gimble: `/Users/tyler/.codex/worktrees/d798/gimble`, starting at `e161721c`.
- SKGO: `/Users/tyler/.codex/worktrees/d798/skgo`, starting at `fed929b`.
- Pinned upstream sources: `/Users/tyler/src/skgo/ephemeral/inspiration/reference`.
- Gimble's currently pinned SKGO and Polytype sources are in the Go module cache
  under `/Users/tyler/go/pkg/mod/github.com/tylergannon/`.

Resolve these five bounded areas from primary source:

1. The smallest one-way package arrangement that removes both the generated
   workflow-to-web dependency and remote-to-host-to-web dependency. Account for
   shared input/result types, SKGO's actual discovery/generation rules, clean
   generation with missing output, the two example commands, and an explicit
   build-time list of stock built-ins. Avoid proposing an executable registry.
2. The precise supported Go client contract for SKGO remote Forms: generated
   identity, enhanced request bytes, typed result/field/error envelopes,
   HTTP-200 errors, validation-only requests, caller cancellation, UDS transport,
   and mutation retry behavior. Map the pinned Kit source and existing independent
   golden fixtures. Identify the actual SKGO release baseline needed by Gimble.
3. Optional/scalar form binding, named/nested fields, fixed model roles and
   defaults: what existing Polytype machinery supports and what SKGO must change.
   Distinguish absence from explicit zero/false/empty. Do not invent a need for
   generic map or file-upload support these workflow inputs do not have.
4. Instance-scoped start admission on both browser and control listeners,
   including headless operation, origin/Referer policy, discovery of an instance
   before project admission, canonical project ownership, run publication,
   conversation association, and request-versus-run lifetime. Identify concrete
   traps in preserving the existing host lifecycle.
5. The smallest usable visible form for each built-in using the actual SvelteKit
   version and existing UI conventions, plus load-bearing verification of shared
   browser/Go calls and one-host multiple-project behavior. Account for existing
   tests, real browser observation, and release/integration sequencing across the
   two repositories. Do not turn the report into a general form-builder design.

Give concrete source references, recommended choices with reasons, and remaining
uncertainties an implementer must resolve. The accepted plan is authoritative;
if source disproves a proposed detail, report the contradiction explicitly.
Separate observed implementation facts from proposed changes. Keep the final
report within 6500 tokens and focus on details not already settled in the plan.

This is research only. Do not edit application code, definitions of done, or
the accepted plan; do not commit, push, publish, or launch another workflow.
Write research notes and annotated source excerpts as Markdown under the supplied
research directory, and the final report at the supplied output path. Do not
copy executable source files, run output, or screenshots into tracked ephemeral
material. Source references must resolve locally for the implementing agents.
