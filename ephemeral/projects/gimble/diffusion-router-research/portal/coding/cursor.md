# Cursor

Source: https://connect.diffusion.io/connect?guide=cursor
Group: Coding | Status: documented | Portal review: 2026-09-22

Point Cursor's desktop Chat and Agent at Diffusion Router with Cursor's own OpenAI base-URL override.

Cursor documents bring-your-own-key for desktop Chat and Agent only. Cursor sends these requests from its own servers, so your Diffusion key passes through Cursor. Tab, the Cursor CLI, and the Cursor SDK still use Cursor's models.

## Prerequisites

- Cursor desktop installed and signed in.
- A Diffusion Router API key from API keys. Use a dedicated key so you can revoke it independently.
- A model ID from the Connect catalog (for example deepseek-4.1-flash).

## Setup

### Add the Diffusion model

Open Cursor Settings, then Models. Add a custom model named deepseek-4.1-flash (or any catalog ID) and enable it.

### Override the OpenAI base URL

Under API Keys, paste your Diffusion key into the OpenAI API Key field, turn on Override OpenAI Base URL, and enter the base URL below. Verify the key when Cursor asks.

```text
https://router.diffusion.io/v1
```

## Restart

Open a new Chat so Cursor picks up the model and key.

## Verification

### Send one prompt through Diffusion

Select deepseek-4.1-flash in the model picker and send "Say ready in one word." The request appears in Activity for your key.

## Troubleshooting

- 401/403 or verification fails: check the key and tenant binding in API keys, and confirm the base URL ends in /v1.
- The override applies to every OpenAI model in Cursor; disable Cursor's built-in OpenAI models while it is on.
- Model not found: the custom model name must exactly match a catalog ID.
- Agent tool calling depends on the model; if Agent stalls, try Chat or another catalog model.

## Removal

### Remove the Diffusion key

Turn off Override OpenAI Base URL, clear the OpenAI API Key field, and delete the custom model. Revoke the key in API keys if unused.

## Rollback

- Re-enable Cursor's built-in models in Settings, then Models.
- No files are modified by this guide.
