# Leaf 05: methodology, validation, release, and proof

## Source coverage and authority

- `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md` is the
  current repository gate: requirements plus observed working behavior decide
  completion; style and taste do not block it.
- `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/research/api/SPRINTS.md`
  is the delivery design/history: Godoc is the API contract, the sprint order
  is historical delivery context, and later sprints are themselves Gimbal runs.
- The two issue-125 reviews are historical independent-review evidence, not a
  standing product contract. They show how to test whether a claim was really
  demonstrated and how a second round closes findings.
- The external skills are methodology references. Repository instructions and
  current code outrank generic advice where they conflict (notably artifact
  handling, wrappers, and release automation).
- The agent-protocol reference describes worktree, temporary-material, and
  checkpoint conventions; those are process guidance, while this leaf is the
  sole requested research output. `/Users/tyler/.agents/skills/agent-protocol/SKILL.md:8-26`

## Proposed teaching topics

### 1. Author workflows: turn requirements into observable claims

- Audience: workflow authors. Decision: begin with the requested behavior and
  its observable success, then write the smallest ordinary-Go workflow that
  can expose it. Do not turn the plan into a command checklist. [importance:
  high]
- Authority: current instruction and gate. Work is done when requested behavior
  is implemented and seen working; code quality opinions can steer but never
  hold it up. `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md:7-13`
- Retrieval hints: `requirements`, `definition of done`, `observable claim`,
  `ordinary Go`, `smallest workflow`, `no wrappers`.
- Coverage: applies to authoring, planning, and validation handoffs; does not
  prescribe a product feature or a new proof framework.

### 2. Author workflows: keep tactics and loop state visible

- Audience: workflow authors. Decision: express research, retries, critique,
  worktrees, validation, and delivery inline. A loop must expose its source,
  context, runner, verifier, state, halt policy, budget, and escalation path;
  missing mechanics should be fixed in code rather than patched with prose.
  [importance: high]
- Authority: current repository design for inline tactics and simple workflows:
  `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/research/api/SPRINTS.md:20-23`;
  methodology reference for loop anatomy and smallest prompts:
  `/Users/tyler/.agents/skills/write-prompts/references/loop-prompts.md:6-21`
  `/Users/tyler/.agents/skills/write-prompts/references/loop-prompts.md:44-47`
- Conflict: generic loop guidance can suggest more machinery; Gimbal's no-wrapper
  rule and current code win. Retrieval hints: `loop anatomy`, `halt policy`,
  `inline tactic`, `prompt surface`, `budget guard`.
- Coverage: authoring and planning. It is a design heuristic unless the current
  API or task explicitly makes a field or stop rule contractual.

### 3. Author workflows: make adaptation derive from recorded evidence

- Audience: workflow authors and planners. Decision: feed the next decision the
  actual child result, command output, and validation state; deterministic
  nonzero evidence must override a worker's prose success. A plan change counts
  only when observed state causes a different useful choice.
  [importance: high]
- Authority: historical review evidence: round 01 rejected a proof where the
  harness supplied the promised failure and the plan did not diverge
  `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/reviews/202609112230-issue125-loop-round-01.md:59-97`;
  round 02 accepted a real failed probe, repair, and deferred nonblocking defect
  `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/reviews/202609112242-issue125-loop-round-02.md:52-86`.
- Retrieval hints: `adaptive dispatch`, `recorded result`, `failed check`,
  `nonblocking defect`, `prose success`, `plan divergence`.
- Coverage: authoring and use. It distinguishes independent evidence from a
  narrative summary; it does not require a particular model or fixture.

### 4. Build/release: use contract order and explicit repository gates

- Audience: maintainers and release operators. Decision: derive implementation
  and release checks from the current Godoc/API contract and the requested
  sprint scope; run the relevant repository checks, then inspect whether those
  checks actually demonstrate the claim. The sprint sequence is a delivery
  ordering aid, not permission to add unrequested surface area.
  [importance: high]
- Authority: current design says Godoc is authoritative, API.md records reasons,
  and later delivery is done by `cmd/sprint`; `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/research/api/SPRINTS.md:1-23`.
- Authority: current gate requires an agent to assess whether automation really
  demonstrates done, and limits validator fixes to invalid validation
  `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md:15-30`.
- Conflict: generic release/process advice may demand extra artifacts or GitHub
  automation; repository instruction says workflow code does not automate
  GitHub `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md:32-39`.
- Retrieval hints: `Godoc contract`, `sprint order`, `validator`, `repository
  checks`, `invalid validation`, `release gate`.

### 5. Build/release: separate required gates, taste, and deferred defects

- Audience: implementers, validators, and release maintainers. Decision: block
  only unmet requirements or invalid proof. Supervisors may object and steer,
  but never gate; once software works and is 90-95% complete, remaining quirks
  become issues rather than hidden release blockers.
  [importance: high]
- Authority: current gate and supervisor contract
  `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md:32-53`.
- Historical evidence: round 02 retained only genuine nitpicks after material
  findings were resolved, while preserving explicit reasons they did not block
  issue #125 `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/reviews/202609112242-issue125-loop-round-02.md:88-117`.
- Retrieval hints: `material finding`, `nitpick`, `supervisor`, `90-95%`,
  `deferred issue`, `release blocker`.
- Coverage: build/release decisions. Never promote a taste preference into a
  requirement without a current authoritative source.

### 6. Use Gimbal: distinguish checks, proof, and live operation

- Audience: users of Gimbal and validators. Decision: tests, vet, compilation,
  and lint are required checks; proof is an observed behavior of the running
  application or workflow. A passing aggregate gate supports a claim but does
  not establish an unrelated runtime behavior.
  [importance: high]
- Authority: external proof reference states checks are not proof unless they
  exercise the whole claimed behavior `/Users/tyler/.agents/skills/proof-of-work/SKILL.md:9-19`;
  repository gate requires an agent to confirm what automation demonstrates
  `/Users/tyler/.codex/worktrees/365e/gimbal/docs/definition-of-done.md:15-27`.
- Historical evidence: the review rejected scaffolded adaptive proof and later
  accepted a real run with failed and repaired probes
  `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/reviews/202609112230-issue125-loop-round-01.md:91-97`
  `/Users/tyler/.codex/worktrees/365e/gimbal/ephemeral/reviews/202609112242-issue125-loop-round-02.md:63-81`.
- Retrieval hints: `live run`, `runtime evidence`, `required check`, `proof
  claim`, `independent validation`, `scaffolded proof`.

### 7. Use Gimbal: review independently, then converge without bureaucracy

- Audience: reviewers and maintainers. Decision: inspect the full target,
  surrounding code, requirements, and available proof; report only concrete
  material findings (or genuine nitpicks), then re-review the entire current
  target after fixes. Consensus stops at no findings or only nitpicks.
  [importance: medium]
- Authority: review methodology `/Users/tyler/.agents/skills/adversarial-review/SKILL.md:13-45`;
  consensus sequence `/Users/tyler/.agents/skills/consensus/SKILL.md:14-21` and
  stop rule `/Users/tyler/.agents/skills/consensus/SKILL.md:37-50`.
- Conflict: these are reviewer procedures, not a universal requirement that
  every small task spawn a review loop. Use them when an independent review is
  authorized and useful; do not add review bureaucracy to routine work.
- Retrieval hints: `full diff`, `concrete reproduction`, `material finding`,
  `same reviewer`, `three exchanges`, `HITL adjudication`.

### 8. Use Gimbal: report evidence and blockers with honest scope

- Audience: release consumers and future maintainers. Decision: report what was
  observed, what remains local or unproved, the exact blocker, and the current
  head/checks; never convert a summary into proof. Prompt and loop closeout
  should name changed surfaces, verifier, before/after behavior, and assumptions.
  [importance: medium]
- Authority: external proof closeout `/Users/tyler/.agents/skills/proof-of-work/SKILL.md:33-43`;
  prompt closeout `/Users/tyler/.agents/skills/write-prompts/SKILL.md:53-61`.
- Conflict: generic proof-upload guidance is subordinate to Gimbal's current
  repository rule that proof is what was seen and no run/proof artifacts are
  committed. Retrieval hints: `proved head`, `unmet claim`, `proof blocker`,
  `before/after`, `local evidence`, `assumption`.
- Coverage: use and release communication; it preserves the boundary between
  legitimate blockers and optional follow-up work.
