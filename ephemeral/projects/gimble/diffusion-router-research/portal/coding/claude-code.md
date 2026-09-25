# Claude Code

Source: https://connect.diffusion.io/connect?guide=claude-code
Group: Coding | Status: beta | Portal review: 2026-09-20

Point Claude Code at Diffusion Router with environment variables; no subscription login is involved.

Full guide qualification on the current production release is pending.

Bearer-token configuration avoids the subscription-login precedence issue. Non-Claude model IDs need explicit selection; Claude Code's native picker filters them.

## Prerequisites

- Claude Code installed.
- A Diffusion Router API key from API keys.

## Setup

### Export your Diffusion key

The auth token is your Diffusion API key.

```shell
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

### Set the Diffusion environment

Add these to your shell profile and open a new terminal.

```shell
export ANTHROPIC_BASE_URL="https://router.diffusion.io"
export ANTHROPIC_AUTH_TOKEN="$DIFFUSION_API_KEY"
export ANTHROPIC_MODEL="deepseek-4.1-flash"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="deepseek-4.1-flash"
export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
```

## Restart

Open a new terminal so Claude Code reads the new environment.

## Verification

### Start Claude Code and send one prompt

The Messages surface is used end to end.

```shell
claude
```

## Troubleshooting

- 401/403: verify the key in API keys and your tenant's active binding.
- Background-task model unavailable: set ANTHROPIC_DEFAULT_HAIKU_MODEL to a compatible catalog model. Gateway picker discovery is documented for Claude Code v2.1.129+, with non-Claude model names selected explicitly.
- Model rejected: non-Claude IDs require explicit selection; pick one from the Connect catalog.
- Subscription login takes precedence: confirm ANTHROPIC_AUTH_TOKEN is exported in the shell you launch from.
- Unknown-model and Advisor warnings can be nonblocking; do not invent behavesAs mappings or context-window overrides. A 400 is a separate failure: verify Router accepts /v1/messages?beta=true and /v1/models?limit=1000.

## Removal

### Unset the Diffusion environment

Remove these exports from your shell profile and open a new terminal. Revoke the key in API keys if unused.

```shell
unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN ANTHROPIC_MODEL ANTHROPIC_DEFAULT_HAIKU_MODEL CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY
```

## Rollback

- Remove the exports from your shell profile — no files are modified by this guide.
- Relaunch Claude Code to return to the previous provider.
