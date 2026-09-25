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

## Execution and observation

Observation may degrade; execution must not. Provider events, Gimbal's
projection of them, and the files and page built from that projection are
diagnostic evidence. Missing, overlapping, duplicated, or unexpectedly ordered
events do not cancel active work and do not replace a harness's authoritative
successful result with failure. Preserve what was actually observed and expose
the gap rather than inventing a settlement or enforcing one the provider did
not promise.

Execution still fails when its authority says it failed: the harness cannot be
started or reached, the provider returns failure, the caller cancels, no
terminal result arrives, or a required result is absent or invalid.

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
res, err := coder.Generate[Result](ctx, task, gimbal.WithSupervisor(taste, "don't let it over-engineer."))
```

With `TYPESAFE_API_KEY` set, each completed thinking message exposed by the
worker's provider sends a separate Jev check for each attached supervisor
instruction. An attachment is one rule for routing; compound instructions
remain one check until they are split at their workflow call sites. Jev
receives the rendered task (up to 20 KiB), that thinking item
(up to 8 KiB), and at most the first 100 characters of each of the last 24
tool-call inputs. Truncation is marked, and Jev receives no tool results.
A check above the provisional 0.65 review threshold asks one coding
supervisor to inspect the packet and the local transcript. A successful look
starts a one-minute review cooldown; a steer that lands starts a two-minute
automatic-steering cooldown. Jev checks continue during both cooldowns.
Provider-exposed reasoning may be only a summary; providers without a
completed-thinking event, and failed Jev checks, use the timed look.

Without the key, the supervisor looks every three minutes, or at its
`WithInterval`, at recent activity. In either mode it steers the worker with
any objection and never gates the result. A supervisor takes the same options
as the turn it watches, so it can have supervisors of its own. Prompt quality
and routing calibration are tracked in
[issue #376](https://github.com/tylergannon/gimbal/issues/376).
