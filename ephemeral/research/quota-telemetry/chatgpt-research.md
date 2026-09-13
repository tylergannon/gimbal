# Quota Telemetry in Google Antigravity and Anthropic Claude Code

## Executive summary

**Yes—both can expose useful remaining subscription-quota telemetry, but Antigravity is easier to query on demand.**

**Google Antigravity (`agy`)** has an official machine-readable quota probe: since CLI v1.1.11, `agy -p "/usage"` and `agy -p "/quota"` work non-interactively and support JSON/NDJSON without starting an agent turn or consuming agent quota. The command refreshes quota state from the backend; Google documents the quota UI as reporting remaining requests/tokens by model. The precise JSON schema for the `/quota` result is, however, **not publicly documented**, so consumers should tolerate field/schema changes. citeturn18view3turn18view1

**Claude Code** now exposes the analogous data officially through its **status-line JSON**. In Claude Code **v2.1.251+**, Pro/Max accounts receive `rate_limits.five_hour` and `rate_limits.seven_day`, each containing `used_percentage` and `resets_at`. Thus remaining quota is `100 - used_percentage`. This is subscription utilization, **not an absolute remaining-token count**; Anthropic does not expose the underlying token allowance in that object. citeturn14view3turn14view6

## Comparison

| Provider | CLI emission | Response headers | Secondary API/CLI | Notes |
|---|---|---|---|---|
| **Google Antigravity** | **Yes:** `agy -p "/quota" --output-format json` | No documented Antigravity weekly/5h quota headers | `/usage`, `/quota`, `/credits` | Best automation surface. JSON quota schema itself is unspecified publicly. citeturn18view3turn18view1 |
| **Anthropic Claude** | **Yes:** `rate_limits` in status-line JSON; `/usage` interactively | `anthropic-ratelimit-*` exist, but describe API RPM/TPM limits, **not Pro/Max 5h/7d quota** | `/usage`; no documented standalone `claude usage --json` or public subscription-usage REST API | Official machine feed is status-line JSON for Pro/Max. citeturn14view3turn14view2turn7search0 |

## Google Antigravity

For an orchestrator, issue a separate zero-agent-turn probe:

```bash
agy -p "/quota" --output-format json
# alias:
agy -p "/usage" --output-format json

# credit overage balance, if relevant:
agy -p "/credits" --output-format json
```

Google's changelog explicitly says these read-only print-mode commands return a structured payload under `--output-format json`/`stream-json`, without creating a conversation or spending quota. `/usage` refreshes model configuration and quota status from the backend; Google describes the panel as showing remaining requests/tokens for each supported model. citeturn18view3turn18view1 Normal agent-stream telemetry remains separate—for example, headless result events expose `usage.input_tokens`, `output_tokens`, `thinking_tokens`, `cache_read_tokens`, and `total_tokens`. citeturn20view1turn20view3

Authentication is simply the CLI's cached Google account session; headless mode requires an authenticated interactive session first. Google also supports `GEMINI_API_KEY`, but that mode explicitly bypasses the Antigravity account session and sends calls directly to the Gemini API, so it should be treated as a different quota domain rather than a way to inspect Google AI Pro/Ultra Antigravity entitlement. citeturn19search0turn20view1

Google documents Pro/Ultra Antigravity quotas as user-plan quotas with five-hour refreshes plus weekly limits; the exact backend accounting key—user, project, credential, etc.—is **not specified** for individual accounts. Enterprise/team usage operates under Google Cloud/Gemini Enterprise Agent Platform terms. No public documentation names an Antigravity HTTP `*-remaining` header or supported quota REST endpoint. citeturn18view2

## Anthropic Claude Code

For Claude Pro/Max, configure a status-line command and capture the JSON Claude Code supplies on stdin:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/.claude/quota-collector.sh"
  }
}
```

The documented payload is:

```json
"rate_limits": {
  "five_hour": {
    "used_percentage": 23.5,
    "resets_at": 1738425600
  },
  "seven_day": {
    "used_percentage": 41.2,
    "resets_at": 1738857600
  }
}
```

So a collector can derive:

```bash
jq '{
  remaining_5h_pct:  (100 - .rate_limits.five_hour.used_percentage),
  reset_5h:          .rate_limits.five_hour.resets_at,
  remaining_7d_pct:  (100 - .rate_limits.seven_day.used_percentage),
  reset_7d:          .rate_limits.seven_day.resets_at
}'
```

These fields appear only after the first API response and are officially documented for Claude.ai **Pro and Max** subscribers; either window can be absent. Claude Code re-invokes the status-line command after assistant messages and when a known quota window reaches its reset time. citeturn14view0turn14view3turn13search0

The ordinary `claude -p ... --output-format json|stream-json` interface documents result/session/usage metadata but **does not document `rate_limits` as part of its headless result schema**. Thus the supported quota side channel is currently the status-line feed rather than the normal agent result stream. citeturn14view2 `/usage` also displays subscription-plan utilization interactively; when its usage request is itself rate-limited, Claude Code can fall back to a snapshot loaded during the preceding 60 minutes. citeturn8view0

Do not confuse this with Claude Platform API headers:

```text
anthropic-ratelimit-requests-remaining
anthropic-ratelimit-requests-reset
anthropic-ratelimit-tokens-remaining
anthropic-ratelimit-tokens-reset
anthropic-ratelimit-input-tokens-remaining
anthropic-ratelimit-output-tokens-remaining
```

Those report API request/token-bucket limits, normally at organization/workspace scope, **not the Claude.ai Pro/Max five-hour or seven-day subscription allowance**. citeturn7search0 Community clients have discovered an OAuth `/api/oauth/usage` endpoint, but Anthropic does not document it as a supported public API; its scopes, stability and polling limits are therefore unspecified, so it is a poor production dependency. citeturn11search12turn11search1

## Implementation guidance

For **AGY**, run `agy -p "/quota" --output-format json` as a secondary probe after/between tasks and cache the result briefly; Google documents the operation as a fresh backend check but publishes no polling-rate or cache-TTL contract. Parse defensively because the structured `/quota` schema is not documented. citeturn18view1turn18view3

For **Claude**, install one status-line collector that writes `.rate_limits` to your daemon/socket/NDJSON file, keyed by account/session; compute remaining percentages locally. Keep raw agent-token telemetry separate because **subscription quota consumption is not specified as a simple token counter**. No extra OAuth scope is needed beyond the Claude Code login when using this official status-line mechanism. For a pure non-interactive runner where status-line execution is unavailable, there is currently **no documented equivalent `claude quota --json` or public subscription-quota HTTP API**; that is the principal capability gap versus `agy`. citeturn14view3turn14view6turn14view2