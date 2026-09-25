# Crush

Source: https://connect.diffusion.io/connect?guide=crush
Group: Coding | Status: beta | Portal review: 2026-09-20

Add an openai-compat provider pointing at Diffusion Router.

Full guide qualification on the current production release is pending.

Configure this client manually using the steps below.

## Prerequisites

- Crush installed.
- A Diffusion Router API key from API keys.

## Setup

### Merge the Diffusion provider

Preserve existing providers and your default model; this block is additive.

Path: `~/.config/crush/crush.json (merge)`

```json
{
  "providers": {
    "diffusion": {
      "type": "openai-compat",
      "base_url": "https://router.diffusion.io/v1",
      "api_key_env": "DIFFUSION_API_KEY",
      "models": [
        { "id": "deepseek-4.1-flash", "context_window": 128000 }
      ]
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

Restart Crush or start a new session so the merged configuration is read.

## Verification

### Select the Diffusion model

Press Ctrl+L or run the model picker and choose the Diffusion entry.

```shell
crush
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Model missing from the picker: confirm the models array merged and the id matches the Connect catalog exactly.

## Removal

### Remove the Diffusion provider

Delete the providers.diffusion block and unset the key. Revoke the key in API keys if unused.

```shell
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.config/crush/crush.json from your pre-edit copy.
- Restart Crush; your previous default provider is untouched.
