Commands are workflow operations alongside agent turns. Record their actual execution so a run can explain which process ran, where, for how long, with what output, and how it ended. #201 discovers command sites in source; that alone cannot say a process ran.

## Result

- Workflow-launched commands have a constant call-site name and an owning scope. Repeated executions have distinct runtime identities, separate from the source template.
- Record executable/arguments, working directory, actual start and end, exit status, cancellation or start failure, and stdout/stderr (or run-local artifact references for large output).
- Preserve ordering and scope placement alongside agent turns. A failed command followed by a successful retry remains inspectable as two attempts; it does not falsely mark unrelated parallel work failed.
- Serve the records through observation for live and finished runs. This task delivers recording and read access; viewer rendering follows #215/#173.
- Keep ordinary os/exec and the execution decision visible in the workflow. Choose the smallest explicit recording mechanism; do not introduce a high-level shell/workflow wrapper or reflection/runtime.Caller. Agree any necessary public API name before adding it.

## Proof

Run a small workflow containing a successful command, a nonzero exit, a command that cannot start, and a cancelled command, followed by an agent inspecting command output. The saved records distinguish all four outcomes and attach them to the correct scope. Repeated and parallel commands remain distinct; reading the finished run after restart returns the same facts. Merely constructing an exec.Cmd produces no execution record.

The scope is commands launched by workflow code, not every shell tool an agent invokes internally. Provider tool calls already have their own transcript path. Do not capture the process environment wholesale.
