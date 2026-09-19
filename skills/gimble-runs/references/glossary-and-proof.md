# Terms and proof

## Planning and execution

| Term | Meaning |
| --- | --- |
| Chapter | Optional long-range direction across multiple sprints: design principles, product direction, and boundaries. It guides planning rather than serving as an executable backlog. |
| Sprint | A bounded plan of executable work and its definition of done. Sprint documents and their ledger track the plan; a run is one execution. |
| Promise | A checkable claim about an outcome or condition. Define scope, standard, evidence, and who or what verifies it. Add freshness rules when the claim depends on changing state. |
| Task | An assignment with a desired outcome and definition of done. A planner can revise the backlog without changing the original goal. |
| Workflow/program | The ordinary Go program describing roles, work, and decisions. |
| Run | One execution of a workflow with recorded activity and results. |
| Scope | A named lifetime within a run, owning context and work. Scopes nest. |
| Role | A cognitive responsibility such as review or planning, assigned a model for the run. |
| Session | An agent conversation for a role. Its exact ID is the target for session steering. |
| Turn | One generation in a session. A session may have many turns. |
| Supervisor | A session and instruction attached to a worker or planner turn. It observes and steers; it is advisory and does not gate completion. |
| Validator | The agent, person, or check assessing acceptance. Gimble's development method uses an agent to judge whether validation actually demonstrates the requested behavior. |

A chapter may guide several sprints. A sprint may contain several tasks and
require several runs. A promise describes what must be established by the
work. These relationships help when that planning method is in use; every
invocation does not need every artifact.

Gimble's planner-driven loop chooses tasks from a goal, backlog, and prior
results. Stopping dispatch does not certify a repository-level promise. The
`df-promise` skill/helper has a separate contract for evidence, verification,
resumption, freshness, and badges. Do not infer those features from a loop's
name.

## What makes evidence useful

Start with the claim and the observation that would support or falsify it.
“The parser accepts this syntax” calls for exercising that parser with the
relevant input. A successful build answers a different question. A claim
about browser interaction needs observation of that interaction.

Tie evidence to the inspected state: project, revision or other subject,
command or observation, result, and relevant limits. Later changes can make
earlier evidence insufficient. For claims intended to remain true over time,
say what invalidates the evidence and when it needs checking again. A written
cadence alone does not establish recurring execution.

Distinguish completed execution, fulfilled goals, failed checks, and blocked
or unproved work. A validator examines the actual result and the legitimacy of
the check. Worker summaries, planner conclusions, supervisor objections, and
steering acknowledgments can guide investigation; they do not substitute for
observing the claimed behavior.
