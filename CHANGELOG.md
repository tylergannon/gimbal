# Changelog

All notable changes to Gimbal will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.13.22] - 2026-10-14

### Changed

- fix(ci): retain Vite+ split directories on Bob runners (#550)

## [0.13.21] - 2026-10-14

### Changed

- Keep the Go cache parent writable in Bob runner containers (#548)

## [0.13.20] - 2026-10-14

### Changed

- Keep the Vite+ guidance beside the commands (#547)

## [0.13.19] - 2026-10-14

### Changed

- Use Vite+ for dependency management and frontend commands (#546)

## [0.13.18] - 2026-10-14

### Changed

- Set up Node before installing pnpm on Bob (#545)

## [0.13.17] - 2026-10-14

### Changed

- Use Bob for persistent Linux release builds and qualification (#544)

## [0.13.16] - 2026-10-14

### Changed

- Run release qualification alongside Linux packaging (#543)

## [0.13.15] - 2026-10-14

### Changed

- fix(release): repair clean native preparation and archive lint (#541)

## [0.13.14] - 2026-10-14

### Changed

- perf(release): reuse builds and overlap qualification (#540)

## [0.13.13] - 2026-10-14

### Changed

- perf(release): keep mise cache local on persistent Mac (#538)

## [0.13.12] - 2026-10-14

### Changed

- feat: retain generators in the shared development supervisor (#534)

## [0.13.11] - 2026-10-14

### Changed

- perf: cache generation and share compatible package analysis (#528)

## [0.13.10] - 2026-10-14

### Changed

- perf: scope release checks and observe feedback budgets (#530)

## [0.13.9] - 2026-10-14

### Changed

- perf(generate): share caller symbols through SKGO and Polytype APIs (#527)

## [0.13.8] - 2026-10-13

### Changed

- perf(test): avoid duplicate context rendering and overlap recovery cases (#526)
- perf(generate): batch workflow loading and reuse compiler exports (#524)
- perf(test): remove polling waits and redundant executable builds (#522)
- Fetch release credentials from guarded Doppler access (#523)
- Add one-time GitHub environment secret copy to Doppler staging (#521)

## [0.13.7] - 2026-10-13

### Changed

- Keep commit hooks limited to fast staged-file checks (#520)

## [0.13.6] - 2026-10-13

### Changed

- fix: let chats start without a committed Git repository (#519)

## [0.13.5] - 2026-10-13

### Changed

- Shrink the app icon from 902 KB to 78 KB (#518)
- docs: record release preparation key and rotation procedure (#517)

## [0.13.4] - 2026-10-13

### Added

- Publish a checked patch release automatically after main changes, including
  signed macOS updates and Linux CLI archives. Retry the same version after a
  publication failure.

### Changed

- Put Projects and Workflows in the native workspace shell, restore the last
  project on launch, and open sessions ready to compose with live replies.
- Add native Project and Recording menu commands, working recording controls,
  and the Gimbal mascot application icon.

## [0.13.3] - 2026-10-12

### Fixed

- Launch the macOS app's bundled Gimbal server when no `GIMBAL_SERVER_URL`
  is supplied. Keep the desktop project registry in `~/.gimbal/desktop`,
  restore it on relaunch, and wait for server readiness before opening the
  workspace. An explicit server URL continues to connect to that server.

## [0.13.2] - 2026-10-12

### Added

- Sparkle updates for the macOS desktop app, with daily background checks,
  a Check for Updates menu, and signed installation with relaunch.

### Fixed

- Strip machine-specific build paths from release CLIs so plugins built in
  another checkout can load. Document the matching plugin build flags and
  source workspace, and check a separate plugin build before publishing assets.

## [0.13.1] - 2026-10-12

### Fixed

- Build releases with Go 1.27.2, which fixes native plugin loading on macOS 27.
  Disable redundant GitHub Go cache transfers on the persistent Mac release runner.
- Declare the macOS CLI's SDK 27 deployment requirement in its Homebrew Cask.

## [0.13.0] - 2026-10-12

### Fixed

- Codex reconciles active turns after connection loss with bounded retries and
  same-run operator resume, preserving completed work and native identity.
  Failed remote cleanup retains stop control; `gimbal cancel` retries it.
  The adapter contract now explicitly owns connection maintenance and
  responsible best-effort completion of requested turns.

### Added

- Release packaging for CGO-enabled macOS and Linux CLI archives, Homebrew
  Casks, and the Developer ID signed, notarized macOS desktop app. The CLI
  reports its version, commit and build date with `--version`.

- Local and Docker task daemons for agent sessions, commands, and scope-owned
  services, with automatic scope cleanup, retained cleanup retries, and the
  `service-check` workflow. The console opens complete captured command output.

- Workflow design pages with source-authored labels, expandable diagrams, prompt
  and context inspection, usage guides, and run forms. URL state preserves
  inspection across reloads; the development server reflects Go source edits live.

- `research-document` checks extracted semantic-index claims against local source
  passages with Jev, checks cross-claim contradictions, marks findings in the
  index, and requires a bounded repair before writing or accepting the document.
  A compiled `audit-index` command performs the check through `RunCommand`.
- `research-eval` compares researcher and index-curator models independently on
  fixed sources, measuring factual grounding, index retrieval, repairs, time,
  and recorded cost before a provisional holdout choice.
- Public compiler integration for typed agent operations, immutable context,
  scope/task lifetimes, graph generation and hosted whole-run cancellation.
  A separate Temporal consumer module demonstrates generation, execution and
  maintenance through those public packages.

- Model aliases stay current with the latest models. `just models` and the daily
  refresh job now also read the Diffusion Router catalog and regenerate the
  aliases `haiku`, `sonnet`, `opus`, `fable`, `luna`, `terra`, `sol`, `astra`,
  `deepseek-flash`, `glm`, `glm-flash`, and `glm-vision`, each pointing at its
  newest version (Claude Sonnet 5.5 included). The refresh job can also be run
  manually and needs the `DOPPLER_TOKEN` repository secret.

- An owned Go Pi coding engine for Diffusion Router runs sessions inside Gimbal,
  with native tools, history, compaction, steering, forks, and live model
  metadata. It replaces the per-session Pi subprocess.

- Generated commands for the five built-in workflows call their matching typed
  SKGO Form endpoints. A running instance admits a project on first start and
  owns accepted runs after the CLI exits. Visible workflow start forms remain
  deferred.

## [0.12.1] - 2026-09-22

### Fixed

- Include the generated web application in the published Go module, so a
  versioned `go install` binary can start its console without a local build.

## [0.12.0] - 2026-09-22

### Added

- One Gimbal instance can host multiple projects with separate live runs,
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
  replacement that prevented installing Gimbal from a published version.
- Show Claude Opus 5.5 consistently in the built-in product workflow help and
  installed Gimbal instructions.

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

- `gimbal upload-artifact` support for Cloudflare R2 and AWS S3, with
  user-wide configuration in `~/.gimbal/config.json` and environment overrides.
- Practical product user-testing workflow with up to three parallel assignments,
  captioned screenshots and human-review videos, Gemini Flash visual review, and
  consolidated findings with optional GitHub issue creation. Testers use Opus 5
  and answer a same-session follow-up on their favorite and least favorite UX/UI
  aspects.
- OpenCode legacy harness with explicit `opencode/model` and
  `opencode/provider/model` routing, one shared server managed by
  `gimbal opencode start|stop`, and raw event/request captures for diagnostics.

[Unreleased]: https://github.com/tylergannon/gimbal/compare/v0.13.22...main
[0.13.22]: https://github.com/tylergannon/gimbal/compare/v0.13.21...v0.13.22
[0.13.21]: https://github.com/tylergannon/gimbal/compare/v0.13.20...v0.13.21
[0.13.20]: https://github.com/tylergannon/gimbal/compare/v0.13.19...v0.13.20
[0.13.19]: https://github.com/tylergannon/gimbal/compare/v0.13.18...v0.13.19
[0.13.18]: https://github.com/tylergannon/gimbal/compare/v0.13.17...v0.13.18
[0.13.17]: https://github.com/tylergannon/gimbal/compare/v0.13.16...v0.13.17
[0.13.16]: https://github.com/tylergannon/gimbal/compare/v0.13.15...v0.13.16
[0.13.15]: https://github.com/tylergannon/gimbal/compare/v0.13.14...v0.13.15
[0.13.14]: https://github.com/tylergannon/gimbal/compare/v0.13.13...v0.13.14
[0.13.13]: https://github.com/tylergannon/gimbal/compare/v0.13.12...v0.13.13
[0.13.12]: https://github.com/tylergannon/gimbal/compare/v0.13.11...v0.13.12
[0.13.11]: https://github.com/tylergannon/gimbal/compare/v0.13.10...v0.13.11
[0.13.10]: https://github.com/tylergannon/gimbal/compare/v0.13.9...v0.13.10
[0.13.9]: https://github.com/tylergannon/gimbal/compare/v0.13.8...v0.13.9
[0.13.8]: https://github.com/tylergannon/gimbal/compare/v0.13.7...v0.13.8
[0.13.7]: https://github.com/tylergannon/gimbal/compare/v0.13.6...v0.13.7
[0.13.6]: https://github.com/tylergannon/gimbal/compare/v0.13.5...v0.13.6
[0.13.5]: https://github.com/tylergannon/gimbal/compare/v0.13.4...v0.13.5
[0.13.4]: https://github.com/tylergannon/gimbal/compare/v0.13.3...v0.13.4
[0.13.3]: https://github.com/tylergannon/gimbal/compare/v0.13.2...v0.13.3
[0.13.2]: https://github.com/tylergannon/gimbal/compare/v0.13.1...v0.13.2
[0.13.1]: https://github.com/tylergannon/gimbal/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/tylergannon/gimbal/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/tylergannon/gimbal/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/tylergannon/gimbal/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/tylergannon/gimbal/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/tylergannon/gimbal/releases/tag/v0.11.0
