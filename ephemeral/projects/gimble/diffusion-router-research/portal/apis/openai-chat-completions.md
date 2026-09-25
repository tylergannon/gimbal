# OpenAI Chat Completions

Source: https://connect.diffusion.io/connect?guide=openai-chat-completions
Group: APIs | Status: locally-tested | Portal review: 2026-09-20

Use any OpenAI SDK or plain HTTP against Diffusion Router.

The Chat Completions relay surface is exercised by Diffusion's transport test matrix.

## Prerequisites

- A Diffusion Router API key from API keys.
- curl or any OpenAI SDK.

## Setup

### Point OpenAI tooling at Diffusion

The OpenAI-compatible base URL ends in /v1, exactly as SDKs expect.

```shell
export OPENAI_BASE_URL="https://router.diffusion.io/v1"
export OPENAI_API_KEY="$DIFFUSION_API_KEY"
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Open a new terminal (or re-create your SDK client) so the environment is read.

## Verification

### One curl exchange

A 200 with a completion body proves the whole path.

```shell
curl "$OPENAI_BASE_URL/chat/completions" \
  -H "Authorization: Bearer $DIFFUSION_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-4.1-flash","messages":[{"role":"user","content":"Say ready."}]}'
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- 413: request bodies are bounded; split very large payloads.
- 429: the upstream rate limit is forwarded with Retry-After — wait and retry; Diffusion never replays automatically.

## Removal

### Unset the environment

Remove the exports from your shell profile. Revoke the key in API keys if unused.

```shell
unset OPENAI_BASE_URL OPENAI_API_KEY DIFFUSION_API_KEY
```

## Rollback

- No files are modified; unsetting the variables returns the SDK to its default endpoint.
