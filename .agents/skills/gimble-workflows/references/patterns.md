# Workflow patterns

These are choices about information and judgment, expressed as ordinary Go in
the workflow. Use Godoc and the current executable examples for API usage.

## Research and requirements

Research is useful when the next role lacks facts it needs. Record the useful
findings and their source locations, then hand them onward. Shared reading
can happen once before candidate conversations diverge; independent judges
must remain able to inspect the original evidence.

An interview resolves material uncertainty about goals or constraints. An
adequate existing specification can be used directly. Do not require the same
interview, planning, critique, and review stages for every task.

## Parallel candidates

```text
research the problem
share the useful context with candidate agents
run independent attempts in separate worktrees
join the attempts
have a judge inspect candidates against the goal
continue with the selected result
```

Parallelism serves independent work. Comparison should use candidate behavior
or artifacts rather than summaries of what the candidates claim to have done.
Keep worktree creation, inspection, and cleanup visible in the workflow.

## Critique and revision

A critic examines a proposal against the requirement and evidence. The writer
accepts or rejects findings with reasons. Keep the goal fixed while improving
the proposal. A material failure may block completion; a taste preference does
not create a requirement.

Bound the rounds or define another appropriate stopping condition. If a real
contract contradiction cannot be resolved from available authority, surface it
instead of quietly changing acceptance.

## Adaptive implementation

```text
establish the goal, constraints, and acceptance evidence
create the planner and scope coach
for each assignment chosen from current evidence:
    have a worker perform the assignment with coaching
    inspect the work and run the relevant validation
    record results, evidence, and unresolved findings
    return that information to the planner
assess the overall goal against the accumulated evidence
```

Tasks describe desired outcomes and useful verified facts. Allow investigation
when it enables a sound next decision. Planning should respond to observed
failures, successful checks, dependencies, and newly discovered facts.

A finite plan can seed this process. Keep deferred defects visible when later
work provides more value than immediately repairing them. Stop when the
contract is fulfilled or the workflow reaches an honest limit or blocker.

## Validation and supervision

A validator examines what was produced and whether the check establishes the
requested behavior. A worker's account is a starting point for inspection.
Record actual results and protect acceptance from being weakened to obtain a
pass.

A supervisor acts during work, while an objection can still influence the
approach. Coach the planner as well as workers and validators where scope
drift matters. Successful message delivery is separate from observed effect.

## Context and continuity

Share research that will actually help the next role. Preserve durable
knowledge in local files so another agent can find it without inheriting the
entire conversation. Use an index to retrieve from a large corpus; keep it a
working aid sized to retrieval, not a project to perfect before doing the work.

Choose session continuity deliberately. It can save repeated orientation, but
also carries stale assumptions. Independent attempts and evaluations need
appropriate isolation, not simply different role labels on the same history.
