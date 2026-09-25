# Authoring route

Use this route to write a workflow that is easy to understand, observe, steer,
validate, and call. Start from the required outcome, not from an API inventory.

| Author's decision | Read next |
| --- | --- |
| How much orchestration belongs in this workflow? How do I write an inline tactic? | [Ordinary Go and executable shapes](../../leaves/01-structure.md) |
| Where should a session live? How do I share context without leaking task state? | [Context, lifetime, prompts and artifacts](../../leaves/02-context.md) |
| Is this finite iteration or adaptive work? How do results influence the next task? | [Planning and feedback](../../leaves/03-planning.md) |
| How do I coach planner, worker and validator without creating approval gates? | [Supervision](../../leaves/04-supervision.md) |
| What does validation need to see, and when should the workflow stop? | [Methodology and proof](../../leaves/05-methodology.md) |
| What is a promise, and how does its contract differ from the runtime loop? | [Promises](../../leaves/08-promises.md) |
| Which research, planning, critique and implementation stages are useful here? | [Delivery loops](../../leaves/10-delivery-loops.md) |
| How do I package a workflow and give it useful generated CLI help? | [Workflow packaging and documentation](../../leaves/11-workflow-packaging.md) |
| Why distinguish fixed program structure, variable task data, and agent knowledge? | [Historical authoring teachings](../../leaves/12-historical-teachings.md) |

The teaching sequence is intent and proof → readable program shape →
session/context ownership → decisions and feedback → coaching → documented
callable workflow. The skill should explain why to choose a pattern, not merely
show a list of primitives. Keep longer runnable examples and API detail in
references reached from the relevant topic.

For operator commands follow the [usage route](../use/index.md). For generated
asset ownership, build checks and installing the result follow the
[release route](../release/index.md). Resolve historical names and unimplemented
ideas through [authority and conflicts](../authority/index.md).
