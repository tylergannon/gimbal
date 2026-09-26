# Recovery review

The remote push history places Claude's continuation on `codex/pi-go-research`,
ending at `dfa333d2`, not a new implementation branch. It rebased the previous
research/planning commits onto `6dc64f83` (the merged Pi RPC harness and model
discovery documentation). Its five new commits added only REQUIREMENTS.md and
HANDOFF.md. No production port or workflow code was changed by that continuation.

Keep the recorded Router-only scope, existing headless-engine functionality,
module ownership, source pin and support-file audit. The main recovery work is
to implement, not restart research. Tyler has explicitly asked for ordinary
translated unit tests and no further planning/test-framework expansion.

Corrections applied to the handoff:

- Parallel package ownership does not isolate Git state or compiling imports.
  Workers use separate worktrees, then serial integration.
- Upstream `packages/ai/test/abort.test.ts` is credential-gated and makes live
  provider calls. It is not the offline mock suite the handoff described.
- `packages/ai/src/compat.ts` describes itself as a temporary global API facade.
  Port the called behavior; do not carry its process-global registry/reset into
  a multi-session embedded Go engine.
- Resources depends on configuration; it cannot be declared ready solely
  because other independent tools are ready.
- The actual Router catalog advertises Pi compatibility metadata, including
  `max_tokens` and model-specific thinking configuration. Generic OpenAI-host
  defaults in upstream source are not the Router's authoritative settings.
- Repeated live prompts are useful practical checks, not exact equivalence
  tests. Workers should primarily translate relevant existing unit tests;
  ordinary integration and real coding runs complete the check.

The merged Pi adapter, binding and model-alias package tests pass locally.
This verifies that baseline, not completion of the native port.

## Model choice

The authenticated catalog for Tyler's key includes full GLM-5.3, GLM Flash and
vision variants, DeepSeek 4.1 Flash and background variants. It does not expose
a non-Flash DeepSeek, Kimi or Qwen chat coder. The earlier phrase "two models"
was misleading: there are two relevant model families with several variants.
A page showing other harnesses' model usage is not the same as this key's
Router catalog.

The [GLM publication](https://z.ai/blog/glm-5.3) and
[DeepSeek publication](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)
make both credible candidates; their reported benchmarks do not establish
performance through our Pi setup. We used a small real truncation-module port,
through Gimbal's merged TypeScript Pi harness, in isolated scratch directories.
DeepSeek 4.1 Flash completed in about 63 seconds and its ordinary unit tests
passed again when rerun by the coordinator. Full GLM-5.3 terminated with a
provider error after about 272 seconds without code; that is an execution
failure, not a coding-quality verdict. DeepSeek background also completed in about 53 seconds; its unit tests passed
again locally. GLM Flash and GLM Vision background both ended with provider
error `terminated` after about 260 seconds. Use DeepSeek background for the
module translations and Sonnet for validation; keep Luna as the authorized
fallback. This small trial establishes a usable starting choice, not a general
model ranking.

Tyler authorizes falling back to the regular Luna coding and Sonnet validation
harnesses if the Router models cannot reliably do the work. No credential is
stored in tracked files.
