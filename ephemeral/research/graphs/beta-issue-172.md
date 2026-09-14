# #172: Embed OpenCode's model prices: filter models.dev, go:embed, daily refresh that tags a patch when the data changes

Captured 2026-09-14 from https://github.com/tylergannon/gimble/issues/172. Read beta-implementation-handoff.md for final session decisions and coordination notes.

## Problem

Gimble has token counts per model call, per turn, per session, and (after the usage coverall) per scope, but a dollar figure only where a harness states one: Claude, once per turn. Codex and Antigravity state nothing. To compare models on a run the page needs a posted price per model, and it should be the same data OpenCode uses so numbers agree with what people see elsewhere.

OpenCode's price data is models.dev. At the pinned revision `c55ee2a8152603f04a409163bd3edf79c425fbd7`, `packages/core/src/models-dev.ts` embeds a snapshot of `https://models.dev/api.json` (`packages/core/src/models-dev/snapshot.txt`, 3.6 MB) and refreshes it from the network. The original token-counting work (#149, #156) left prices out; this puts them in without the network.

What `api.json` holds today (4.6 MB, 213 providers): per model, `cost.input`, `cost.output`, `cost.cache_read`, `cost.cache_write` in USD per million tokens, plus `release_date` and `last_updated`. Some OpenAI rows carry `tiers` for long context (gpt-5.6 above 272k). There is no reasoning price; providers bill reasoning tokens as output. Rows exist for every model Gimble has run:

| table id | input | output | cache read | cache write | last_updated |
|---|---|---|---|---|---|
| `claude-haiku-4-5-20251001` | 1 | 5 | 0.1 | 1.25 | 2025-10-15 |
| `claude-fable-5-1` | 10 | 50 | 0.25 | 12.5 | 2026-09-01 |
| `claude-opus-5` | 5 | 25 | 0.5 | 6.25 | 2026-07-24 |
| `gpt-5.6-luna` | 0.2 | 1.2 | 0.02 | 0.25 | 2026-07-09 |
| `gpt-5.6-sol` | 4 | 20 | 0.4 | 5 | 2026-07-09 |
| `gemini-3.8-flash` | 0.75 | 3.75 | 0.075 | none | 2026-09-02 |

Antigravity reports `gemini-3.8-flash-low`; the `-low`/`-medium`/`-high` suffix is Gimble's effort encoding (`internal/modelalias`), and the table key is `gemini-3.8-flash`.

## Ask

1. **A filter program.** One ordinary Go program (under `internal/`, private, run by `go generate` or a `just prices` recipe, decide which and say) that downloads `https://models.dev/api.json`, keeps the providers Gimble's harnesses serve (`anthropic`, `openai`, `google`), keeps per model only `id`, `name`, `release_date`, `last_updated`, and the four base `cost` fields (drop `tiers`; say so in the file header), sorts deterministically, and writes one small JSON file beside a header recording the download date. Deterministic output: the same upstream data yields a byte-identical file.
2. **Embed it.** The file is compiled into the binary with `go:embed`. A private lookup takes the model string a harness reports and returns the row or nothing: exact id first, then the id with a trailing `-low`, `-medium`, or `-high` removed. Nothing else is guessed. No new exported names.
3. **Daily refresh.** A GitHub Actions workflow on a daily cron (and `workflow_dispatch`) runs the filter on main. If the filtered file is byte-identical, it stops. If it changed, it commits the file to main and tags the next patch version. Gimble is a Go module; the tag is the release. There is no release process or tag today, so this issue also picks the first tag (propose `v0.1.0` before the first automated `v0.1.1`) and says that a human-made tag bumps minor or major.
4. **The consumer** is the usage coverall's cost proxy (see the coverall issue): per-model tokens times these prices, applied once in the fold, unknown model shown as "no price", never $0.

## Not asked

- No runtime download, no cache, no network at run time.
- No quota or subscription arithmetic: the price is a comparison proxy for API-billed tokens.
- No tiers, no per-region pricing, no currency other than USD.

## Proof

- Run the filter locally: the committed file is byte-identical to a second run on the same day, and its rows for the six models above match this issue's table.
- Run the workflow by dispatch with the file already current: it exits without a commit or a tag. Change one price in the committed file by hand and dispatch again: it restores the upstream value, commits, and tags the next patch. Show both runs' logs.
- `go test ./...` covers the lookup: exact id, the stripped effort suffix, and an unknown id returning nothing.
