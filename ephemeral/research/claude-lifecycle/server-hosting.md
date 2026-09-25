# Follow-up: multi-session servers and process count

Update: the [Generate-lifetime experiment](generate-lifetime.md) demonstrated
that a new process can resume the same conversation with a different schema or
no schema. Prefer retaining the process through one logical Generate, rather
than the whole Session, when live work need not span calls. Earlier session-wide
recommendations below are superseded by this narrower option.


Checked 2026-09-20 against current official documentation and the installed
Claude Code 2.1.270 binary. Earlier wording that no shared daemon/server exists
was too broad. Claude has two relevant supervisors, but neither establishes
multiple independent conversations executing inside one worker process.

- [Agent SDK hosting](https://code.claude.com/docs/en/agent-sdk/hosting)
  explicitly maps one concurrent session to one CLI subprocess.
- [Agent view supervisor](https://code.claude.com/docs/en/agent-view#the-supervisor-process)
  owns separate per-session Claude processes. It retires eligible unattended
  idle processes after about an hour and resumes their saved conversations.
- [Remote Control server](https://code.claude.com/docs/en/remote-control)
  supports concurrent sessions, with same-dir/worktree modes and default
  capacity 32. Local `claude remote-control --help` confirms these flags even
  though the command is absent from the top-level help in this installation.
  Documentation describes sessions served from one process; this refers to
  the server entrypoint, not elimination of per-session workers.

The installed binary at
/Users/tyler/.local/share/claude/versions/2.1.270 contains embedded JavaScript.
Its bridge session factory imports `spawn as qr` from `child_process`, constructs
arguments including `--print`, `--sdk-url`, `--session-id`, and stream-json input
and output, then calls `qr(e.execPath, R, {cwd:n, stdio:["pipe","pipe","pipe"], ...})`.
It records the resulting child PID per session. The inspected factory begins
at byte offset 186491482 approximately (locate by
`function rr(e){return{spawn(r,n)`; offsets are build-specific). This is static
inspection of the distributed executable, not a live multi-session benchmark.

Remote Control is documented for Claude web/mobile clients through Anthropic's
backend. No supported local general-purpose session RPC interface usable as a
Gimbal replacement was established. The native `--sdk-url` validation explicitly
reserves that transport for Remote Control workers connecting to Anthropic's
backend; do not assume it is a general local WebSocket endpoint.

Different response schemas are a separate question. If each Generate call must
retain exact native schema enforcement, the tested persistent process cannot
switch between those types. A stable native completion envelope with host-side
validation of each call's payload could preserve shared conversation history
across differently typed call sites, but changes the enforcement contract and
still needs a prototype. It would use one process per live session, not per call
site or schema. Retiring a process is safe only after defining what live work
and events must survive; conversational idle alone does not establish that.
