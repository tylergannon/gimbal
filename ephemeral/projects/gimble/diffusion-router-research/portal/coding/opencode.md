# OpenCode

Source: https://connect.diffusion.io/connect?guide=opencode
Group: Coding | Status: beta | Portal review: 2026-09-20

Register Diffusion Router as an OpenCode provider with your Diffusion API key.

Full guide qualification on the current production release is pending.

This guide registers Diffusion Router directly using OpenCode’s provider configuration and your Diffusion API key.

## Prerequisites

- OpenCode installed and currently working.
- A Diffusion Router API key from API keys.

## Setup

### Merge the Diffusion provider

Preserve existing providers and plugins; this block is additive.

Path: `~/.config/opencode/opencode.json (merge)`

```json
{
  "provider": {
    "diffusion": {
      "models": {
        "deepseek-4.1-flash": {}
      }
    }
  },
  "providers": {
    "diffusion": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "https://router.diffusion.io/v1",
        "apiKey": "{env:DIFFUSION_API_KEY}"
      }
    }
  }
}
```

### Export your Diffusion key

Add the export to your shell profile and open a new terminal.

```shell
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Restart OpenCode so the merged configuration and provider plugin load.

## Verification

### Discover models through Diffusion

The catalog lists the native upstream model IDs as served by Diffusion.

```shell
opencode models
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Provider missing after restart: confirm the JSON merged cleanly (no trailing commas, valid JSON).
- Empty catalog: Diffusion reports discovery state on the Connect page; retry when it is fresh.

## Removal

### Remove the Diffusion blocks

Delete both the provider and models blocks you merged and unset the key. Revoke the key in API keys if unused.

```shell
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.config/opencode/opencode.json from your pre-edit copy.
- Restart OpenCode; other providers and plugins are untouched.
