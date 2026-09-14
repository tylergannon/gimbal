# Definition of done

How work here is gated, for the agents doing it and for the workflows
they run. Tyler set these rules on 2026-09-10, after the first live run
of the sprint workflow.

## Development is gated on the requirements

The development side is gated entirely on realizing the requirements, not
on code quality. A piece of work is done when what it asks for is
implemented and has been seen working. Opinions about style, structure,
or taste may steer the work while it happens (see Supervisors); they
never hold it up.

## Validation

The validation side is an agent. It looks at the validation and makes
sure it actually demonstrates the definition of done, and that no other
agent has tampered with it or otherwise made it illegitimate. What it
produces is a reliable claim that the requested functionality is
implemented and has been seen working.

- Automated validation (tests, commands) is fine, but an agent still
  confirms that the automation demonstrates what the definition of done
  requires.
- The validator may run manual checks of the running application to
  validate.
- The validator may request fixes only where the validation is
  legitimately invalid. Validation is not a blank check to ask for
  enhancements, or for fixes to bugs that do not prevent validation.

## Exit and merge

Once the software actually works and is 90-95% complete, the agents
driving the workflow exit and merge. Remaining quirks and bugs are filed
as GitHub issues.

The agents file issues and open and merge pull requests themselves, with
the tools they already have. Workflow code does not automate GitHub.

## Supervisors

A supervisor is a session and an instruction, attached to one turn where
the turn is started:

```go
res, err := coder.Generate[Result](ctx, task, gimble.WithSupervisor(taste, "don't let it over-engineer."))
```

Every three minutes, or at its `WithInterval`, it looks at what the worker
did since its last look and steers the worker with any objection. It
never gates the result. A supervisor takes the same options as the turn
it watches, so it can have supervisors of its own. See Supervisors in
`ephemeral/research/api/API.md`.
