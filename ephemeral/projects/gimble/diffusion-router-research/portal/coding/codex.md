# Codex

Source: https://connect.diffusion.io/connect?guide=codex
Group: Coding | Status: beta | Portal review: 2026-09-23

Use Diffusion in Codex CLI with a profile, or make it the default for new local ChatGPT Desktop tasks.

Full guide qualification on the current production release is pending.

Codex CLI 0.155.0-alpha.16.3 returned a short Responses reply with glm-5.3-flash through production Diffusion Router on 2026-09-23. A native Desktop task and a full coding workflow remain unverified. Model discovery failed during the CLI check, so Codex used fallback model metadata. DeepSeek currently rejects Codex input_text content upstream.

## Prerequisites

- Codex installed; check `codex --version` and keep it for the record.
- A Diffusion Router API key from API keys.
- For Desktop, use a local task on the same computer as the Codex configuration files.

## Setup

### Register the provider

Merge this block into your user-level Codex configuration and keep existing providers. The file names the key variable; it does not contain the key.

Path: `~/.codex/config.toml (merge; keep existing providers)`

```toml
[model_providers.diffusion]
name = "Diffusion Router"
base_url = "https://router.diffusion.io/v1"
wire_api = "responses"
env_key = "DIFFUSION_API_KEY"
supports_websockets = false
```

### Create the CLI profile

Use this profile when launching Codex CLI. It does not change the Desktop composer by itself.

Path: `~/.codex/diffusion.config.toml (new file)`

```toml
model_provider = "diffusion"
model = "glm-5.3-flash"
```

### Load the key for Codex CLI

Add the export to your shell profile and open a new terminal. Keep the key out of the TOML files and your repository.

```shell
export DIFFUSION_API_KEY="dfr_v1_YOUR_DIFFUSION_KEY"
```

### Select Diffusion for ChatGPT Desktop

For new local Desktop tasks, set these top-level values in ~/.codex/config.toml, before any [table]. This changes the default for new Codex sessions on this computer; the CLI profile alone does not select the Desktop model.

Path: `~/.codex/config.toml (top level; merge with existing settings)`

```toml
model_provider = "diffusion"
model = "glm-5.3-flash"
```

### Load the key for ChatGPT Desktop

Desktop may not inherit your shell profile. Put the same personal key in this local file and run `chmod 600 ~/.codex/.env`. If the file exists, add this line without replacing its other values. Never commit it.

Path: `~/.codex/.env`

```shell
DIFFUSION_API_KEY=dfr_v1_YOUR_DIFFUSION_KEY
```

## Restart

CLI: open a new terminal, then launch the profile. Desktop: quit and reopen ChatGPT Desktop, then start a new local Codex task. Existing tasks keep their selected model.

## Verification

### Check CLI, then Desktop

Run the CLI profile and send a short prompt. In Desktop, start a new local task after restart and send a short prompt; confirm that it uses Diffusion and glm-5.3-flash. A successful CLI request alone does not verify the Desktop task.

```shell
codex --profile diffusion
```

## Troubleshooting

- 401/403: check the key in API keys and that your tenant holds an active binding.
- Responses 404: report it to Diffusion support with `codex --version`. Do not switch to wire_api="chat" unless Diffusion confirms it for your build.
- Profile not found: confirm both files exist and the profile name matches exactly.
- Desktop still uses the previous provider: put model_provider and model at the top level of ~/.codex/config.toml, restart the app, and start a new local task.
- Desktop cannot find the key: check the DIFFUSION_API_KEY entry in ~/.codex/.env; a shell export alone may not reach the app.
- Model catalog refresh reports request unavailable: try a short inference request. Codex can use fallback metadata, but that warning is not proof that a full coding workflow works.

## Removal

### Remove the Diffusion provider and profile

Restore the previous top-level provider and model in ~/.codex/config.toml, delete the [model_providers.diffusion] block, remove the profile, and remove only the DIFFUSION_API_KEY line from ~/.codex/.env and your shell profile. Restart Desktop after changing its configuration.

```shell
rm ~/.codex/diffusion.config.toml
unset DIFFUSION_API_KEY
```

## Rollback

- Restore ~/.codex/config.toml from your pre-edit copy.
- Remove the Diffusion key line from ~/.codex/.env if you added it for Desktop, and remove the shell export if you added it for CLI.
- Restart Desktop and start a new local task, or launch CLI without the profile, to confirm the previous setup still works.
