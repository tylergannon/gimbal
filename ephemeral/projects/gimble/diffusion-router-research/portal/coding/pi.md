# Pi

Source: https://connect.diffusion.io/connect?guide=pi
Group: Coding | Status: beta | Portal review: 2026-09-20

Configure Pi's custom OpenAI-compatible provider to talk to Diffusion Router with a Diffusion key.

Full guide qualification on the current production release is pending.

Diffusion is qualifying the exact Pi release through Diffusion Router. Configure Pi directly with your Diffusion API key.

## Prerequisites

- A Diffusion Router API key from API keys (create one if you have none).
- Pi installed and working locally before any configuration change.
- A model ID from the Connect catalog (for example deepseek-4.1-flash).

## Setup

### Add Diffusion as a custom provider

Register an OpenAI-compatible provider that points at Diffusion Router. Use your Diffusion API key.

Path: `~/.pi/agent/models.json (merge; preserve existing keys)`

```json
{
  "providers": {
    "diffusion": {
      "baseUrl": "https://router.diffusion.io/v1",
      "api": "openai-completions",
      "apiKey": "$DIFFUSION_API_KEY",
      "models": [{ "id": "deepseek-4.1-flash" }]
    }
  }
}
```

### Export your Diffusion key in your shell profile

Add the export to ~/.zshrc or ~/.bashrc, then open a new terminal.

```shell
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

### Choose a model

List models through Diffusion and select one. Model IDs are the native upstream IDs shown in the Connect catalog.

```shell
curl -H "Authorization: Bearer $DIFFUSION_API_KEY" https://router.diffusion.io/v1/models
```

## Restart

Fully quit Pi and start it again so the merged configuration is read.

## Verification

### Send one headless prompt through Diffusion

A completion proves the full path: Pi, Diffusion auth, relay, upstream.

```shell
pi -p --provider diffusion --model "deepseek-4.1-flash" "Say ready in one word."
```

## Troubleshooting

- 401/403: the key is wrong, revoked, or your tenant's Diffusion Router access is suspended. Check API keys for key state and allocation.
- Model missing: add its exact catalog ID to providers.diffusion.models in models.json, then reopen /model.
- If your Pi build rejects the provider block, keep the block and tell Diffusion support your exact Pi version.

## Removal

### Remove the Diffusion provider

Delete the providers.diffusion block from the config file and unset the environment variable. Revoke the key in API keys when it is no longer used.

```shell
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.pi/agent/models.json from the copy you made before editing.
- Restart Pi; previous providers are untouched because the guide only merges.
