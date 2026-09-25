# OpenAI Responses

Source: https://connect.diffusion.io/connect?guide=openai-responses
Group: APIs | Status: locally-tested | Portal review: 2026-09-20

Use the Responses surface with any Responses-capable client.

The Responses relay surface is exercised by Diffusion's transport test matrix.

## Prerequisites

- A Diffusion Router API key from API keys.
- curl or a Responses-capable SDK.

## Setup

### Point the client at Diffusion

Same environment as Chat Completions; the surface is selected by the path.

```shell
export OPENAI_BASE_URL="https://router.diffusion.io/v1"
export OPENAI_API_KEY="$DIFFUSION_API_KEY"
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Open a new terminal (or re-create your SDK client) so the environment is read.

## Verification

### One curl exchange

A 200 with a response body proves the surface end to end.

```shell
curl "$OPENAI_BASE_URL/responses" \
  -H "Authorization: Bearer $DIFFUSION_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-4.1-flash","input":"Say ready."}'
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Streaming stalls: Diffusion relays SSE verbatim; check client proxy buffering.

## Removal

### Unset the environment

Remove the exports from your shell profile. Revoke the key in API keys if unused.

```shell
unset OPENAI_BASE_URL OPENAI_API_KEY DIFFUSION_API_KEY
```

## Rollback

- No files are modified; unsetting the variables returns the SDK to its default endpoint.
