# GitHub Copilot CLI

Source: https://connect.diffusion.io/connect?guide=copilot-cli
Group: Coding | Status: beta | Portal review: 2026-09-20

Configure the Copilot CLI gateway to use Diffusion Router.

Full guide qualification on the current production release is pending.

Beta integration. The Anthropic alternative carrier (x-api-key) is also documented.

## Prerequisites

- GitHub Copilot CLI installed.
- A Diffusion Router API key from API keys.

## Setup

### Set the gateway environment

Add these to your shell profile and open a new terminal.

```shell
export COPILOT_PROVIDER_TYPE="openai"
export COPILOT_PROVIDER_BASE_URL="https://router.diffusion.io/v1"
export COPILOT_PROVIDER_API_KEY="$DIFFUSION_API_KEY"
export COPILOT_MODEL="deepseek-4.1-flash"
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Open a new terminal so the Copilot CLI reads the new environment.

## Verification

### Ask Copilot one question

A completion proves the gateway is using Diffusion Router.

```shell
copilot
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Requests still hit github.com: unset the variables in the shell you launch from and retry.
- Anthropic carrier alternative: set COPILOT_PROVIDER_TYPE=anthropic and COPILOT_PROVIDER_BASE_URL without /v1; the relay accepts x-api-key on /v1/messages.

## Removal

### Unset the gateway environment

Remove all five exports from your shell profile. Revoke the key in API keys if unused.

```shell
unset COPILOT_PROVIDER_TYPE COPILOT_PROVIDER_BASE_URL COPILOT_PROVIDER_API_KEY COPILOT_MODEL DIFFUSION_API_KEY
```

## Rollback

- Remove the exports — no files are modified by this guide.
- Relaunch Copilot CLI to return to the default gateway.
