# Gimbal Codex desktop worker

An experimental local worker that creates and observes Codex desktop tasks through
an ordinary stdio MCP plugin. Its socket client supports `status`, `start`, `read`,
`message`, and `cancel`. It is not a Gimbal `HarnessAdapter`: it does not implement
streaming events, schema-constrained turns, fork, or reliable active-turn steering.

The desktop bridge is private, version-dependent, and tied to a loaded owner task.
The MCP packaging is supported; the bridge itself is not a public OpenAI API or an
OpenAI-supported Gimbal integration. Desktop updates can break it. Revalidate after
updates. It needs macOS, Node 22+, a signed-in running Codex desktop, and its supplied
`CODEX_APP_TOOLS_PIPE_PATH` and `CODEX_MCP_NODE_PATH` environment variables. No API
key is required. There is no network listener or SSH transport.

## Provision once

1. Install this folder as the `gimbal-codex-desktop` plugin from a local marketplace.
   It uses the supported `.codex-plugin/plugin.json` and `.mcp.json` compatibility
   layout. For a repository marketplace, merge this entry into
   `.agents/plugins/marketplace.json` at the repository root:

   ```json
   {
     "name": "gimbal-local",
     "plugins": [{
       "name": "gimbal-codex-desktop",
       "source": {"source": "local", "path": "./plugins/codex-desktop"},
       "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
       "category": "Productivity"
     }]
   }
   ```

   Trust the project or fixture folder when using project configuration. Install
   from the desktop Plugins Directory. Local install/update may require a fresh
   desktop session or restart; the host loads a cached copy. See the
   [official packaging instructions](https://developers.openai.com/plugins/build/plugins).
2. Enable the plugin. Its MCP configuration sets persistent `approve` policy for
   **only `bind_worker`**, with `prompt` as the default. This is the scoped worker
   provisioning authorization; it does not approve every desktop action. If a host
   override is needed, use the installed plugin's exact ID:

   ```toml
   [plugins."gimbal-codex-desktop@gimbal-local".mcp_servers.gimbal_desktop.tools.bind_worker]
   approval_mode = "approve"
   ```

3. In a dedicated local desktop task, invoke `gimbal_desktop.bind_worker` once with
   `{}`. Codex supplies the actual task and turn metadata. Keep that owner task
   loaded and its MCP process alive. The worker is unavailable when either is
   unloaded; a persistent ledger alone does not keep it alive. Rebind from the
   same owner after recovery, and inspect any stale socket before removing it.
4. Separately, during explicit one-time provisioning, open each actual native
   app through Computer Use and choose **Always allow** in its permission prompt.
   Review or revoke saved access in **Settings → Computer Use**. Complete any required macOS
   permissions through the OS. The plugin reads saved app approvals and never
   writes the native consent store or clicks approval dialogs. Plugin approval
   does not grant native-app consent.
5. Run `status` with the actual app bundle identifiers required by the task. Its
   `ready` value means the worker is bound and those identifiers have saved
   persistent approval. It is not a TCC, accessibility, screen-recording, app
   availability, sensitive-action, or end-to-end computer-use guarantee. An empty
   `requiredApps` list establishes no native-app readiness.

The default state directory is `~/.gimbal/codex-desktop`; set
`GIMBAL_DESKTOP_DIR` in the MCP process environment to override it. Use that same
directory in clients. The directory and `worker.sock` are private to your OS user;
local processes running as that user can submit work. Keep its `sessions.json`
ledger: it records ownership and start identities across worker restarts. One
owner uses each directory. `read`, `message`, and `cancel` reject tasks absent from
that ledger.

## Call the worker

From this plugin directory, pipe one JSON request line into the client:

```sh
node client.mjs --timeout-ms 60000 <<'JSON'
{"method":"status","requiredApps":["com.apple.TextEdit"]}
JSON

node client.mjs <<'JSON'
{"method":"start","requestId":"textedit-check-001","prompt":"Use TextEdit to inspect the open document and report its title.","model":"gpt-5.6-luna","target":{"type":"projectless"},"requiredApps":["com.apple.TextEdit"]}
JSON
```

Use `--state-dir /absolute/path` to override `GIMBAL_DESKTOP_DIR` for a client.
`--timeout-ms` defaults to 60000. It bounds the socket exchange and never triggers
a retry. Prompts that depend on remote requirements should point to their locally
saved files by absolute path.

| Method | Request fields | Behavior |
| --- | --- | --- |
| `status` | Optional `requiredApps` array of bundle IDs | Bound owner, saved approvals, readiness scope, and request ledger |
| `start` | `requestId`, `prompt`, `target`; optional `model`, `thinking`, `requiredApps` | Creates one task; defaults to `gpt-5.6-luna` |
| `read` | `threadId` | Recent native task state and outputs |
| `message` | `threadId`, `prompt` | Sends a follow-up; active-turn steering is not guaranteed |
| `cancel` | `threadId` | Archives the task and verifies that it is unloaded with no observed running turn |

`target` is the desktop `create_thread` target object. For an existing project,
use its real desktop project ID, for example
`{"type":"project","projectId":"YOUR_PROJECT_ID","environment":{"type":"local"}}`.
The worker does not discover project IDs or invent branches. The selected model
and reasoning effort must be supported by the desktop host.

Responses are one line: `{"result":...}` or
`{"error":{"code":"...","message":"..."}}`. The client preserves that envelope
on stdout. Worker errors, invalid protocol responses, socket failures, and timeouts
also produce a stderr diagnostic and a nonzero exit status.

Use a stable `requestId` for each logical start. Repeating the same request returns
the same task; changing its fields returns `request_conflict`. If creation was
attempted but no stable identity was recorded, the worker returns `outcome_unknown`
and does not create another task. Inspect the desktop and ledger before recovery;
do not replace the request ID to bypass an uncertain outcome. Client disconnects
and timeouts do not cancel work.

`read` treats a task waiting on approval as failed unattended work: it attempts
archive-and-stop, then returns `approval_required` with the observed state and
`cancellationConfirmed`. If cancellation was not confirmed, retry `cancel`.
Provision the specific permission manually before new work. This is an intentional
side effect of reading an unattended task, not a passive viewer. The worker never
approves prompts. There is no background monitor: callers must keep reading while
a task runs. `requiredApps` is a declared preflight list, not a restriction on which
apps an agent may later attempt to use. `cancellation_unconfirmed` retains the task for another `cancel`
attempt; an archive request alone is not evidence of termination. Cancelling a
task also archives it, which is stronger than interrupting a turn.

## Validate without restarting the desktop

Run the shipped stdio executable directly to check MCP initialization and tool
discovery. Supply the absolute Node runtime path provided by desktop; the launcher
deliberately fails if it is absent and does not fall back to `node` on `PATH`.
Replace `/absolute/path/to/plugin` and `/absolute/path/to/codex-node` below:

```sh
CODEX_MCP_NODE_PATH=/absolute/path/to/codex-node \
  /absolute/path/to/plugin/scripts/launch <<'JSON'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"stdio-check","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
JSON
```

For a live no-restart check, register the **same shipped executable** as an ordinary
MCP server in a trusted disposable project's `.codex/config.toml`, then open that
project in a fresh desktop task:

```toml
[mcp_servers.gimbal_desktop]
command = "/absolute/path/to/plugin/scripts/launch"
env_vars = ["CODEX_APP_TOOLS_PIPE_PATH", "CODEX_MCP_NODE_PATH", "HOME", "GIMBAL_DESKTOP_DIR"]
default_tools_approval_mode = "prompt"

[mcp_servers.gimbal_desktop.tools.bind_worker]
approval_mode = "approve"
```

Invoke `bind_worker` there through the actual MCP tool so the desktop supplies
genuine executor metadata. Do not fabricate owner metadata or modify first-party
plugin registrations. If this desktop version does not supply its private pipe to
ordinary MCP, report that limitation. This validates the executable/bridge path;
it does not establish that marketplace discovery, installation, or reload works.

Run the package tests with `npm test` in this folder. Fake socket tests establish
protocol behavior, not live desktop or native-app access.
