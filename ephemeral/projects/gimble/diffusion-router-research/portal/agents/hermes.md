# Hermes

Source: https://connect.diffusion.io/connect?guide=hermes
Group: Agents | Status: documented | Portal review: 2026-09-20

Add a Diffusion provider with the chat-completions transport.

Hermes's `hermes config set` rewrites YAML and drops comments; edit the file by hand instead. Generate examples from the live catalog, not static examples.

## Prerequisites

- Hermes installed.
- A Diffusion Router API key from API keys.

## Setup

### Merge the Diffusion provider by hand

Edit the YAML directly to preserve comments; do not use hermes config set.

Path: `~/.hermes/config.yaml (merge; preserve comments)`

```yaml
providers:
  diffusion:
    api:
      base_url: "https://router.diffusion.io/v1"
    key_env: "DIFFUSION_API_KEY"
    transport: "chat_completions"
```

### Export your Diffusion key

Add the export to your shell profile and open a new terminal.

```shell
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

## Restart

Start a new Hermes session so the merged configuration is read.

## Verification

### Select the Diffusion model

Choose the custom provider model; --global persists the selection.

```shell
hermes model custom:diffusion:deepseek-4.1-flash --global
```

## Troubleshooting

- 401/403: check the key and tenant binding in API keys.
- Provider unknown: confirm the YAML indentation merged cleanly and Hermes was restarted.
- Comments lost: you used hermes config set — restore your backup and edit by hand.

## Removal

### Remove the Diffusion provider

Delete the providers.diffusion block by hand and unset the key. Revoke the key in API keys if unused.

```shell
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.hermes/config.yaml from your pre-edit copy.
- Run `hermes model` to reselect your previous default.
