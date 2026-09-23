# Changelog

All notable changes to Gimble will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.12.0] - 2026-09-22

### Added

- One Gimble instance can host multiple projects with separate live runs,
  conversations, controls, and durable history. Multiple configured instances
  can coexist; one active instance owns a given project at a time.
- Compiled workflow commands submit to the selected instance, with separate
  project ownership and execution workdir. Conversations launch workflows
  through that same CLI and retain their run association across restarts.
- The `implement` workflow walks an ordered list of outcomes, using a bounded
  PromiseLoop and independent validation within each outcome.

### Fixed

- A newly published hosted run is observed live without rereading its
  incomplete directory. Hosted workflow panics fail their run without ending
  the instance or unrelated runs.
- The web listener starts only after its initial projects are admitted, and
  project ownership lasts until active runs and conversations finish shutdown.

## [0.11.1] - 2026-09-22

### Fixed

- Use the directly importable Claude SDK fork v1.1.1, removing the module
  replacement that prevented installing Gimble from a published version.
- Show Claude Opus 5.5 consistently in the built-in product workflow help and
  installed Gimble instructions.

## [0.11.0] - 2026-09-22

### Changed

- Default model selections now use GPT-6 Sol/Luna and Claude Opus 5.5 where
  those tiers are configured. The `gpt` shorthand now selects GPT-6 Sol.

### Added

- The `opus` shorthand selects Claude Opus 5.5, with explicit Opus 5.5 and 5
  choices. The GPT family supports explicit versions 6 and 5.6.

### Fixed

- Product evaluators reopen saved screenshots before captioning or citing them,
  ground captions in visible evidence, and carry independent screenshot-review
  corrections into final triage (#323).
- Selecting a watcher in the run graph shows its recorded result and states
  explicitly when no watcher turn was recorded (#324).
- Claude `Generate` waits through background-task continuations before returning
  the final value, and subsequent calls resume the conversation with their own
  output schema.
- The run page's detail pane no longer opens on a wall of assignment text. Tool
  input and output render with real newlines, each payload wraps and scrolls on
  its own, and a wide payload cannot drag the pane sideways (#325).

### Changed

- `TurnStarted.Prompt` records the prompt a workflow passed to `Generate`. The
  scope values sent with it are listed separately in `TurnStarted.Context` (key,
  owning scope, and whether the complete value was sent). What an agent is sent
  is unchanged; runs saved earlier open as before.

### Added

- A session view on the run page: every turn of one agent session in a list,
  the chosen turn's full detail beside it, and steer and stop. Open it with
  the pane's Open session button, `o`, or a double-click on an agent node;
  leave with Close or Esc. The URL does not change (the session route is
  #342).
- Detail pane view control: drag to resize, maximize over the map with `\`, a
  remembered width, and an overlay sheet on narrow windows.
- An agent turn's detail is tabbed: Activity while it runs, Result once it has
  ended, Prompt with its scope context apart, Usage, and Source. Stop turn sits
  beside Steer and stops the selected turn. A scope's detail shows its
  assignment, its context with what each value shadows, and a loop's decisions.
- Storybook stand-ins for remote functions, so the whole run page renders and
  can be driven without a server.

- `gimble upload-artifact` support for Cloudflare R2 and AWS S3, with
  user-wide configuration in `~/.gimble/config.json` and environment overrides.
- Practical product user-testing workflow with up to three parallel assignments,
  captioned screenshots and human-review videos, Gemini Flash visual review, and
  consolidated findings with optional GitHub issue creation. Testers use Opus 5
  and answer a same-session follow-up on their favorite and least favorite UX/UI
  aspects.
- OpenCode legacy harness with explicit `opencode/model` and
  `opencode/provider/model` routing, one shared server managed by
  `gimble opencode start|stop`, and raw event/request captures for diagnostics.

[Unreleased]: https://github.com/tylergannon/gimble/compare/v0.12.0...main
[0.12.0]: https://github.com/tylergannon/gimble/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/tylergannon/gimble/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/tylergannon/gimble/releases/tag/v0.11.0
