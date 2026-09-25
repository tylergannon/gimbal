# Anthropic Messages

Source: https://connect.diffusion.io/connect?guide=anthropic-messages
Group: APIs | Status: locally-tested | Portal review: 2026-09-20

Use the Anthropic-compatible Messages surface with x-api-key or Bearer auth.

The Messages relay surface is exercised by Diffusion's transport test matrix.

## Prerequisites

- A Diffusion Router API key from API keys.
- curl or any Anthropic SDK.

## Setup

### Point Anthropic tooling at Diffusion

The Anthropic-compatible base URL has no /v1 suffix; clients append their own paths.

```shell
export ANTHROPIC_BASE_URL="https://router.diffusion.io"
export ANTHROPIC_API_KEY="$DIFFUSION_API_KEY"
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Open a new terminal (or re-create your SDK client) so the environment is read.

## Verification

### One curl exchange

A 200 with a content block proves the surface end to end.

```shell
curl "$ANTHROPIC_BASE_URL/v1/messages" \
  -H "x-api-key: $DIFFUSION_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-4.1-flash","max_tokens":16,"messages":[{"role":"user","content":"Say ready."}]}'
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Bearer also works on Messages; x-api-key matches the native Anthropic client.

## Removal

### Unset the environment

Remove the exports from your shell profile. Revoke the key in API keys if unused.

```shell
unset ANTHROPIC_BASE_URL ANTHROPIC_API_KEY DIFFUSION_API_KEY
```

## Rollback

- No files are modified; unsetting the variables returns the SDK to the Anthropic API.
