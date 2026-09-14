# Source path in pinned archive: README.md
# Archive member: diffusioninc-Tenacious-d3a4d7ff9f5445a42a7516eda845f8c58a531a45/README.md

# Tenacious

An orchestrator that drives a multi-step workflow across `claude` and
`codex` CLI agents, with per-step assets, outcomes, and an optional
auth-bank for parallel credentials.

## Prerequisites

- Python 3.11+
- The `claude` CLI (Claude Code) on your `$PATH`
- The `codex` CLI on your `$PATH` (only if your workflow uses codex steps)
- A populated auth bank at `~/.tenacious/auth-bank/` — see
  [`POPULATING_THE_AUTH_BANK.md`](POPULATING_THE_AUTH_BANK.md) — *or* the
  `--costly-use-*-api-key` flags below for any provider you'd rather bill
  directly through an API key, or `--codex-home` / `--claude-code-home`
  for a provider whose normal CLI home should be used directly.

## Install

Tenacious's workflow format is YAML. Legacy JSON workflow files still load
for compatibility, but new workflows should be written as `.yaml` or
`.yml`. PyYAML is the only runtime dependency. Run it out of a dedicated
venv:

```sh
git clone <this repo> ~/dev/Tenacious
cd ~/dev/Tenacious
python3 -m venv .venv
./.venv/bin/pip install -r requirements.txt   # PyYAML for workflow loading
```

Workflow files should use `.yaml`/`.yml`. `.json` is accepted as a legacy
format with the same schema.

## Run

Invoke the package as a module, pointing at a workflow file and the
project directory the agents should operate in:

```sh
PYTHONPATH=~/dev/Tenacious \
  ~/dev/Tenacious/.venv/bin/python -m tenacious \
  ~/dev/Tenacious/workflows/6step.yaml \
  --project-dir /path/to/your/working/dir
```

- `workflows/6step.yaml` — the bundled iterative development workflow.
  `workflows/6step-goal-only.yaml` starts the same loop from a supplied
  goal instead of a requirements interview, `workflows/iterative.yaml`
  is a checklist-driven variant, and `workflows/hello-claude-fable5.yaml`
  is a one-step Claude Fable 5 smoke test. Workflow documents define
  steps, agents, models, context, assets, and transitions.
- `--project-dir` — the directory the agents read from and write
  code into. Orchestration assets live separately under
  `~/.tenacious/runs/<run-id>/`.
- `--max-visits N` — override the run-wide cap on step entries plus
  retry visits. The cap is persisted into the frozen `workflow.json`, so
  a resumed run inherits it unless you override it again.

Run `python -m tenacious --help` for the full flag list.

## Cost-bearing API-key mode (optional)

The default mode leases credentials from `~/.tenacious/auth-bank/` so each
agent invocation runs against whatever subscription/OAuth account that
slot represents. Two optional top-level flags let you skip the bank for a
provider and bill that provider's API account directly instead. Each
child run is **billed per token** against the key, hence "costly":

| Flag | Effect |
|---|---|
| `--costly-use-anthropic-api-key` | Bypass the anthropic auth-bank lease. Reads `ANTHROPIC_API_KEY` from the orchestrator's env and exports it to the child `claude` process. `CLAUDE_CODE_OAUTH_TOKEN` is **actively stripped** from the child env so the metered API is billed, not the OAuth plan. |
| `--costly-use-codex-api-key` | Bypass the openai/codex auth-bank lease. Reads `CODEX_API_KEY` (preferred) or `OPENAI_API_KEY` from the orchestrator's env, and exports the selected key to the child `codex` process as **both** `CODEX_API_KEY` *and* `OPENAI_API_KEY`. |

With the flag set, the required env var must be present at launch. The
auth bank is not consulted at all for that provider — no slot leased, no
`auth.json` materialized in the step profile, no `accounts.json` entry for
that provider — and the run can launch with an empty/absent
`auth-bank/<provider>/` tree. When in this mode the orchestrator writes one
banner to `run.log` at run start:

```
[API-KEY MODE: anthropic — billed per token]
[API-KEY MODE: codex — billed per token]
```

If the required env var is missing, Tenacious exits before creating a run;
remove the API-key flag to use the auth bank for that provider. This keeps
the provider in exactly one mode: auth bank, API key, or user home.

The flags are independent: set one, the other, both, or neither. They
are also accepted by `--resume RUN_ID` (and only affect steps that run
after the resume). The full per-flag mechanics, including the exact
run-log banner strings, live in
[`USER_MANUAL/03-running-workflows.md`](USER_MANUAL/03-running-workflows.md).

## User-home auth/session mode (optional)

Two flags let a run use an existing agent home directly instead of the
auth bank or API-key mode:

| Flag | Effect |
|---|---|
| `--codex-home DIR` | Bypass the openai/codex auth-bank lease and run child `codex` processes with `CODEX_HOME=DIR`. Codex auth state and rollout sessions stay under that directory. |
| `--claude-code-home DIR` | Bypass the anthropic auth-bank lease and run child `claude` processes with `CLAUDE_CONFIG_DIR=DIR`. Claude Code auth state and JSONL sessions stay under that directory. |

Tenacious still writes orchestration metadata under
`~/.tenacious/runs/<run-id>/`, including per-step thread/session ids and a
`session-index.jsonl` that points the viewer and health monitor at the
external transcript files. These flags cannot be combined with the
costly API-key flag for the same provider.

## Viewer

A read-only terminal viewer for run state lives in `tenacious/viewer/`.
Launch the four-level TUI (runs → run → step → transcript; Enter goes
deeper, Esc backs out):

```sh
PYTHONPATH=~/dev/Tenacious \
  ~/dev/Tenacious/.venv/bin/python -m tenacious.viewer
```

Non-interactive flavors of the same command (useful while a run is
in flight):

```sh
python -m tenacious.viewer --list                              # one line per run
python -m tenacious.viewer --run <run-id> --info               # run summary
python -m tenacious.viewer --run <run-id> --step code --visits
python -m tenacious.viewer --run <run-id> --step code --sessions
```

See `USER_MANUAL/06-viewer-tui.md` and `07-viewer-cli.md` for the full
docs. The TUI runs list includes status, project, TLDR, visit path, git
progress, plan checklist progress, summed cost when available, and the
auth mode/account used by each provider.
