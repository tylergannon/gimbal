# OpenClaw

Source: https://connect.diffusion.io/connect?guide=openclaw
Group: Agents | Status: beta | Portal review: 2026-09-20

Merge a Diffusion provider into the OpenClaw gateway configuration.

Full guide qualification on the current production release is pending.

Preserve channels, plugins, and your existing default model; the merge mode keeps them intact.

## Prerequisites

- OpenClaw installed with an existing working configuration.
- A Diffusion Router API key from API keys.

## Setup

### Merge the Diffusion provider

The models.mode merge value keeps existing models; JSON5 comments are preserved when the file uses them.

Path: `~/.openclaw/openclaw.json (merge; preserve JSON5 if present)`

```json
{
  "models": {
    "mode": "merge",
    "providers": {
      "diffusion": {
        "baseUrl": "https://router.diffusion.io/v1",
        "apiKey": "{env:DIFFUSION_API_KEY}",
        "api": "openai-completions",
        "models": [
          { "id": "deepseek-4.1-flash" }
        ]
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

Restart the OpenClaw gateway so the merged provider loads.

## Verification

### List models through the gateway

The Diffusion entry must appear next to your existing models.

```shell
openclaw models list
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Models missing: confirm mode is merge (not replace) and the JSON/JSON5 still parses.
- Default model changed unexpectedly: set your previous default explicitly after merging.

## Removal

### Remove the Diffusion provider

Delete the models.providers.diffusion block and unset the key. Revoke the key in API keys if unused.

```shell
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.openclaw/openclaw.json from your pre-edit copy.
- Restart the gateway; channels and plugins are untouched by this guide.
