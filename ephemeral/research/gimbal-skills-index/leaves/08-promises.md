# 08 — Promises

## Retrieval card

- **Audience:** workflow authors, builders/releasers of Gimbal, and people using
  Gimbal to make agent work observable and steerable.
- **Authority:** the current `df-promise` skill and helper define the shipped
  Promise Loop contract and mechanics; the legacy Five Arts and direction
  documents are historical design material and explicitly provisional.
- **Use this leaf for:** teaching what a promise means, how proof earns or
  loses trust, how bounded work resumes, and how to express those ideas in an
  ordinary Go workflow without treating the helper as a Gimbal builtin.
- **Source boundary:** `df-promise` is a skill plus Python helper. These
  sources do not establish it as a Gimbal API or runtime primitive.

## Vocabulary and semantics
- A **Promise Loop** is repository-attached, bounded goal work that accumulates
  evidence across resumable runs and earns a README badge only after every
  fulfillment gate passes. A badge attests to a reviewed commit, subject and
  mechanism fingerprints, evidence, verifier, and time; it is not timeless
  repository health. [.agents/skills/df-promise/SKILL.md:7-18]
- A promise must state an exact, falsifiable condition, scope, standard,
  freshness window, evidence, and authority. Broad labels such as “secure” or
  “SOC 2” are not acceptable claims by themselves. [.agents/skills/df-promise/SKILL.md:20-24]
- **Configured**, **executed**, **fulfilled**, **badge-issued**, **scheduled**,
  and **human-accepted** are different states and must be reported separately.
  [.agents/skills/df-promise/SKILL.md:315-322]
- A cadence is not a runner. The contract names who invokes the run, in what
  environment, with what mutation/external-call authority, and what economics
  apply. [.agents/skills/df-promise/SKILL.md:162-174]
- **Coverage** is the latest result per subject; **evidence** is a durable path,
  hash, URL, or provider/request reference attached to a material event;
  **verification** is the named mechanism that must match the contract.
  [.agents/skills/df-promise/SKILL.md:241-272]

## Create / setup expectations
- Setup starts with an interview, not a badge: resolve claim/scope,
  evidence/truth and staleness, execution authority, economics/budgets, and
  publication semantics before writing the contract. [.agents/skills/df-promise/SKILL.md:152-178]
- Narrow the claim until available evidence can prove or falsify it. The model
  example is “no unresolved high-severity findings in the reviewed inventory at
  fingerprint X,” rather than “the repository is secure.” [.agents/skills/df-promise/SKILL.md:176-178]
- The setup plan names inventory, gate evidence, tools, verifier, invalidation,
  runner, economics, budgets, and scheduling. Declared tools are small,
  deterministic, promise-local programs; the final contract and tools are
  validated before a badge-bearing run. [.agents/skills/df-promise/SKILL.md:192-215]
- Schema 2 requires runner, evidence mode, verifier, and economics. Migration
  from schema 1 is explicit, cannot happen with an active run, and must not
  invent missing cost or approval answers. [.agents/skills/df-promise/SKILL.md:109-113]

## Run / resume / verify expectations
- Start validates the contract, reads recent state/coverage, fingerprints the
  committed subject and mechanism, and rejects dirty scoped work by default.
  `--allow-dirty` is an explicit exception recorded in the run and badge.
  [.agents/skills/df-promise/SKILL.md:241-253]
- “Continuous” means repeated bounded invocations with persisted state, not an
  immortal process. Each invocation finishes or records a checkpoint; the next
  start resumes the active run. Overlapping runs are avoided and state mutation
  is serialized. [.agents/skills/df-promise/SKILL.md:226-239]
- Work proceeds as inventory → inspect → record → assess. Every material
  inspection, test, finding, blocker, decision, and verification gets a record;
  every passing gate needs valid durable evidence. [.agents/skills/df-promise/SKILL.md:254-267]
- Finish supplies exactly one result for every contract gate and one verdict:
  `fulfilled`, `not-fulfilled`, `blocked`, or `budget-exhausted`. Missing runtime
  totals are recorded as partial/unavailable, never invented. [.agents/skills/df-promise/SKILL.md:265-279]
- Fulfillment additionally requires recorded evidence for all gates, an exact
  verifier identity, unchanged subject and mechanism fingerprints, non-empty
  scope, and respected budgets. A failed or non-fulfilled run must remain
  visible; retries cannot conceal it. [.agents/skills/df-promise/SKILL.md:274-279]
- Resume accounting closes the interrupted active segment at the last activity
  and excludes idle time; record a final checkpoint before yielding. [.agents/skills/df-promise/SKILL.md:274-276]

## Trust, freshness, and invalidation
- External or mixed evidence needs a freshness deadline and a durable reference
  such as a content hash, provider request ID, signed report, or stable URL.
  [.agents/skills/df-promise/SKILL.md:162-165; .agents/skills/df-promise/SKILL.md:217-224]
- A changed scoped subject, contract, or promise-local tool invalidates the
  attestation; an age deadline can do so too. A failed gate or later
  `not-fulfilled` run withdraws it. A blocked/budget-exhausted run may preserve
  a still-valid prior attestation. [.agents/skills/df-promise/SKILL.md:281-298]
- Evidence-only commits under `.promises/` and edits to the managed badge block
  do not change the subject fingerprint. This is the helper’s trust model, not
  a general rule that any run artifact is safe to commit. [.agents/skills/df-promise/SKILL.md:291-298]
- Human sign-off is a distinct gate; automated output cannot silently stand in
  for a person. Approval references are recorded, but the helper cannot prove
  the external authority behind them. [.agents/skills/df-promise/SKILL.md:69-74]

## Costs, bounds, and escalation
- A full run declares a cost band, estimate basis, positive hard cap when
  required, and bounds for wall time, steps, tool calls, retries, and tokens.
  Bands `100-1k` and above (and `unknown`) require per-run approval; `10k-100k`
  and above also require staging. [.agents/skills/df-promise/SKILL.md:180-190]
- Cost is checked at checkpoints. An over-cap run records a blocked blocker and
  finishes `budget-exhausted`; it does not continue fulfillment work. [.agents/skills/df-promise/SKILL.md:186-190]
- Use Gimbal’s normal workflow structure to make these bounds visible at the
  call site: explicit stages, loops, decisions, validation, supervision, and
  exit conditions. The historical direction calls page-or-two pseudocode a
  comprehension target, not a hidden framework requirement.
  [ephemeral/legacy/docs/direction.md:102-120]

## What this means for Gimbal workflows

- Teach a user that proof is behavioral evidence tied to a declared goal and
  gate, not a planner’s success text. The Five Arts put “completion follows
  evidence rather than assertion” under ensuring the agent actually does the
  work. [ephemeral/legacy/docs/five-arts.md:16-21]
- Put acceptance and validation where a reader can see them in the Go program;
  keep retries, critique, research, and delivery choices inline enough that the
  workflow’s meaning survives inspection. The direction says source defines
  execution and a derived view must not invent dependencies.
  [ephemeral/legacy/docs/direction.md:102-135; ephemeral/legacy/docs/direction.md:315-321]
- Use bounded scopes, explicit checkpoints, independent checking, and
  supervisor steering to make promise work legible. Steering is an observation
  channel, not authority to manufacture a pass: the Five Arts warn that an
  observer can talk an agent into a verdict it has not earned.
  [ephemeral/legacy/docs/five-arts.md:42-45; ephemeral/legacy/docs/five-arts.md:47-60]
- Preserve the distinction between a workflow’s orchestration state and proof
  of fulfillment. The `df-promise` skill explicitly says a goal loop is
  orchestration state, not proof. [.agents/skills/df-promise/SKILL.md:251-253]
- Historical direction treats telemetry for bad runs as a proposed information
  set, not a first-version event schema, and says the first Go version does not
  add comprehensive telemetry or an eval platform. Do not teach those as
  shipped Gimbal promise features. [ephemeral/legacy/docs/direction.md:244-250; ephemeral/legacy/docs/direction.md:330-342]

## Conflict / gotcha for this repository

- `df-promise` recommends tracked `.promises/` contracts, state, coverage,
  evidence, runs, and events, with compact durable artifacts in the repository.
  [.agents/skills/df-promise/SKILL.md:115-150]
- The Gimbal repository’s contribution contract forbids committing run/proof artifacts.
  This restriction concerns work in this repository; other consumer projects
  retain their own artifact policies. In Gimbal itself, do not import `.promises/`, badges, run logs, or proof programs unless
  the user explicitly changes that repository contract. Preserve the consumer
  project’s artifact boundary; do not “fix” the conflict by silently importing
  the generic helper.
- The badge is historical and commit-pinned, not a self-refreshing CI signal.
  A user of Gimbal must say what was actually run and observed, with model,
  verifier, gates, freshness, blockers, and bounds, rather than infer runtime
  behavior from a green aggregate check. [.agents/skills/df-promise/SKILL.md:281-289]

## Retrieval hints

- For **meaning and proof**: start with the Promise definition, interview
  decisions, run/resume checklist, and badge truth. [.agents/skills/df-promise/SKILL.md:7-24; .agents/skills/df-promise/SKILL.md:152-190; .agents/skills/df-promise/SKILL.md:241-298]
- For **helper mechanics**: inspect `promise.py` evidence-reference rules,
  start/resume fingerprinting, record gate checks, and finish enforcement.
  [.agents/skills/df-promise/scripts/promise.py:236-255; .agents/skills/df-promise/scripts/promise.py:1055-1159; .agents/skills/df-promise/scripts/promise.py:1173-1245; .agents/skills/df-promise/scripts/promise.py:1368-1473]
- For **workflow authoring**: read the direction’s pseudocode, context, and
  source/runtime-view sections; treat Five Arts as competing live priorities,
  not a settled checklist. [ephemeral/legacy/docs/direction.md:102-188; ephemeral/legacy/docs/direction.md:284-328; ephemeral/legacy/docs/five-arts.md:3-14]
