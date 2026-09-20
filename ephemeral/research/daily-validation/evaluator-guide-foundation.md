# Practical user testing: documentation foundation

Status: agreed intent, recorded 2026-09-20 before the next live evaluation.
This note is intended to become both external usage documentation and an
internal reference for evaluating Gimble itself. It is not a claim of proof.

## The contract

The TEST HARNESS arranges a user experience; the TARGET must actually deliver
that experience. Testing a product means using it for its intended purpose.
There is no substitute: replacing the assignment with an easier activity can
produce convincing artifacts while falsifying the conclusion.

1. The workload must require the TARGET's intended capability. For Gimble,
   that means orchestrating agents to perform actual new work.
2. The EVALUATOR accomplishes the assignment through the TARGET: initiates
   work, experiences execution, interacts with the product, and assesses the
   delivered result. Reviewing historical runs is a different use case.
3. The OUTER AGENT and TEST HARNESS preserve the assignment. Neither silently
   simplifies it, performs it on the evaluator's behalf, or bypasses the TARGET.
4. Source blindness applies to every primary product A under test. Its source
   must never inform the evaluator's assessment; public user docs are allowed.
   When A operates on another project B, B's source is permitted, but the
   evaluator should normally rely on A. A's implementation agents may inspect B.
5. Harness completion and target success are separate outcomes. A completed
   evaluation may honestly discover that the target failed. A polished report
   does not establish that the required user journey happened.
6. Evidence supports the assignment; it does not redefine it. Screenshots,
   recordings, elapsed time, agent turns, and reports cannot replace actual use.
   Duration follows the real work, not a desired completion time.
7. Interventions remain visible: instruction repairs, model changes, and
   recovery assistance belong in the account. Delivery of steering is not proof
   of compliance, nor proof that the agent could not have recovered unaided.

## External documentation: how to use the evaluator

Assign practical work as in a human user-testing session. Ask a neutral user to
accomplish a useful task and report usability, obstacles, and success. Avoid
exhaustive test scripts that duplicate unit or conventional E2E tests.

Prepare the desired product build, authenticated model providers,
`playwright-cli` and its browser, and one to three isolated workload directories.
For each workload provide a local assignment file describing the useful outcome,
permitted actions, and completion expectation. Provide local product user guides.
Use JSON or YAML with `product`, `guides`, `output_dir`, and `workloads`; each
workload supplies `name`, `assignment_file`, `workdir`, and `url`. Optional
foreground `start` and readiness `ready` commands let the harness own a service.
Paths resolve relative to the suite file. The current CLI help is authoritative:

```sh
gimble run validate-product --help
gimble run validate-product --suite-file /absolute/path/suite.yaml --no-web
```

The harness runs assigned tasks, then asks each same evaluator session for an
overall assessment and its three favorite and least favorite aspects of UX and
UI separately. Concrete observations matter; do not pad lists. Gemini Flash
opens screenshots to check readability and caption accuracy. A final agent
synthesizes findings. Keep this simple, flat, and explicit.

Screenshots should be ordered and captioned at meaningful moments. Video is for
optional human review. The output directory contains task reports, screenshots,
recordings, measured task durations, visual review, and final findings. Debrief
time is separate from task time. A failed agent turn is an execution error;
finding a product defect is a valid evaluation outcome.

Omit `issue_repo` for reports only. Set it to an authorized `owner/repository`
to publish actionable deduplicated product findings. Downstream project defects
do not belong in the target's issue tracker. The current workflow supports one
issue destination; downstream findings otherwise remain report-only.

## Internal reference: using the evaluator on Gimble

```mermaid
flowchart TD
    U["User: defines testing objective"]
    O["OUTER AGENT: prepares and runs evaluation"]
    H["TEST HARNESS: Gimble evaluation workflow"]
    E["EVALUATOR: Opus 5 acting as a user"]
    T["TARGET: Gimble being evaluated"]
    C["Implementation agents: orchestrated by TARGET"]
    B["WORKLOAD PROJECT: new Scrabbler functionality"]
    R["Results: task outcome, UI/UX feedback, evidence"]
    U --> O
    O -->|"Workload and configuration"| H
    H -->|"Practical assignment"| E
    E -->|"Uses CLI and web UI"| T
    T -->|"Plans and delegates"| C
    C -->|"Build and validate"| B
    B -->|"Evaluator checks delivered behavior"| E
    T -->|"Live progress, controls, results"| E
    E -->|"Observations and same-session debrief"| H
    H -->|"Screenshot review and synthesis"| R
    R --> O
```

Gimble is both harness implementation and target here, with distinct
responsibilities. Scrabbler supplies a real purpose for using Gimble. Its
delivered feature demonstrates whether Gimble helped accomplish the task;
the evaluator's principal UI/UX assessment concerns Gimble.

The outer agent prepares an isolated B worktree and materializes the issue
locally. Keep the evaluator assignment outside B and pass only the feature
definition to Gimble's implementation agents, avoiding recursive delegation.
Use Opus 5 for evaluation and Sonnet or stronger for implementation and
implementation validation. Never use a cheap builder merely to shorten a smoke.

The evaluator launches the target implementation workflow with its own web
listener, omitting `--no-web`, and uses that listener for live observation.
A standalone history viewer can retain stale snapshots of externally owned
runs. Keep the real target process alive while observing; its terminal result
resolves stale-view disagreements. Preserve `.git` and `.gimble`.

The evaluator delegates coding and review, observes/interacts with Gimble during
execution, exercises the delivered feature, and opens a PR when authorized.
It does not inspect Gimble source or implement Scrabbler itself. The outer agent
observes rather than taking over. Any assistance must be reported.

## Next authorized exercise and the remaining proof gap

Run one workload: Scrabbler issue #1, drag-and-drop tiles, through to a PR.
Use the existing evaluator workflow, capture the real experience, and preserve
the user's separate public Scrabbler preview. No merge of the feature PR is
part of this assignment.

Earlier Scrabbler trials exercised real orchestration. The later short Opus
trial inspected recorded runs and proved same-session debrief plumbing only.
The complete Opus-led new-feature journey remains unproven until this exercise
actually happens. Report separately: harness execution, target delivery,
overall Gimble UX/UI, evidence limitations, and outer-agent interventions.

## Intended documentation destinations

- External: a task-oriented evaluator usage guide covering preparation, suite
  inputs, invoking the workflow, interpreting outcomes and artifacts, and issue
  publication. Use a concise example without this conversation's history.
- Internal: retain the role diagram, non-substitution contract, Gimble-on-Gimble
  setup, known monitoring pitfalls, and an honest account of live-run lessons.

Fold this note into those destinations after incorporating the upcoming run's
observations. No official documentation has been changed by this note.
