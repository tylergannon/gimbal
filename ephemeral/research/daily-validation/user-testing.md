# Practical agent user testing

Tyler's direction: treat this like a focus group of software engineers evaluating
Gimble for potential use. Agents should be neutral, carry out useful work they
would otherwise do by hand or with another tool, and report candidly on both task
success and the experience. A short observation checklist is enough. Do not
recreate deterministic E2E or unit tests as elaborate agent instructions.

## Sequence

1. Prepare the product and one or more practical workloads. Put any issue text
   and other essential inputs in local files before dispatch. Each workload names
   the task and what finishing it means, the target project, available tools and
   skills, and allowed actions (for example, deliver a pull request).
2. Fan out one user-testing agent per workload with isolated target state. Each
   uses Gimble for the heavy lifting, monitors it through the web interface,
   explores relevant controls naturally, and captures screenshots of important
   screens, working interactions, and failures. Screenshots are ordered and
   captioned. Video is an optional human-review artifact, not agent input.
3. Each agent reports task completion, concrete output such as a PR, bugs or
   blockers, incorrect/missing/illegible/out-of-place information, and up to three
   most troublesome aspects of the experience. Record elapsed time from workflow
   timestamps. Keep task success and UX quality distinct; success does not mean
   the experience was good. Do not invent three complaints if there are fewer.
4. Gemini Flash opens the screenshots (or an identified subset), checks visible
   usability/readability and whether images support their captions, and flags
   mismatches or uncertainty. This is a visual evidence check, not a second
   execution of the task or proof of behavior the screenshots cannot show.
5. A high-level agent reads the workload reports and visual checks, groups related
   findings, checks existing GitHub issues, and opens actionable issues with
   observed impact, reproduction context, and accessible evidence. It should
   distinguish confirmed defects, UX complaints, and unsupported suspicions.

## Boundaries

For every product under test (A), the user-testing agent must never inspect A's
source code. Test through its public interface, skills, and user documentation.
If A's source is encountered accidentally, ignore it and exclude it from the
assessment. This is a general rule, not a Gimble-specific restriction.

When A is Gimble, a workload may involve doing work in another project (B). The
tester may inspect B's source, but should not normally need to: it should delegate
the work to Gimble and trust Gimble to perform it. Agents Gimble dispatches to
implement the workload can work in B's source. The user-testing agent evaluates A
from the experience and observable outcome rather than doing A's job itself.

Keep the [feature inventory](feature-inventory.md) for code inspection and unit,
integration, E2E, and build-test coverage. It can suggest varied workloads, but is
not a 63-item focus-group script. Initially assess task completion and UI/UX in the
same user session; separate them into distinct runs only if experience warrants it.

This records the revised design. The current feature-by-feature validate-product
implementation does not yet implement workload fan-out, Flash screenshot review,
or final GitHub issue triage. No runs or GitHub issues are created by this note.
