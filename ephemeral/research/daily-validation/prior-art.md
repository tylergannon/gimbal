# Prior art: agent-driven validation of CLI and browser products

Research date: 2026-09-19. Produced through Gimble's `research-document` workflow with Gemini, then edited against current primary documentation and Gimble source. These tools were researched, not exercised as an integrated validator in this task. The [source index](prior-art-final-sources/INDEX.md) preserves the research trail; [checked sources](checked-sources/INDEX.md) correct errors found in the automated notes. The [proposal](proposal.md) makes the recommendation.

## Useful tools and ideas

| Prior art | Documented capability | What to borrow for Gimble | Limitation or tradeoff |
|---|---|---|---|
| [Playwright CLI](https://github.com/microsoft/playwright-cli) | Browser actions, accessibility snapshots, named sessions, code execution, traces, and video start/stop. | Let the existing agent use shell commands to explore the actual product and record the same session. | Needs its package and browser installed. A session can outlive an individual CLI command, so explicit session cleanup matters. |
| [Playwright MCP](https://github.com/microsoft/playwright-mcp) | Browser control through MCP, including recording configuration/tools. | An alternative when the agent harness already has a suitable MCP connection. | An installed Playwright package does not configure MCP. Tool access must be verified in the actual harness. |
| [Playwright Test Agents](https://playwright.dev/docs/test-agents) | Planner, generator, and healer roles for creating and maintaining browser tests. | Explore the running application before describing a test; use observations to produce repeatable checks. | Automatically repairing tests is a separate task. Validation should preserve failures and the intended behavior. |
| [Vercel agent-browser](https://github.com/vercel-labs/agent-browser) | Browser CLI with compact element references, interaction commands, screenshots, traces, and video recording. | Another small driver for a shell-capable agent; compare it if Playwright CLI proves awkward. | Its recording instructions require ffmpeg. Test the installed version and selected browser rather than assuming every documented command is available. |
| [Stagehand](https://github.com/browserbase/stagehand) | SDK operations for natural-language actions, observing elements, and structured extraction. | Its observe-then-act pattern makes actions inspectable and separates element discovery from entering sensitive data. | Adding another model-facing SDK is more integration than the first Gimble workflow needs. |
| [Browser Use](https://github.com/browser-use/browser-use) | An agent framework for browser tasks. | Observe, act, inspect the resulting state, and continue toward a stated outcome. | It introduces another agent loop alongside Gimble's existing harness sessions. |
| [ttyd](https://github.com/tsl0922/ttyd) | A real terminal served in a browser, with bind-address, port, writable-mode, and command-exit signal options. | A candidate for driving and filming CLI interaction with the same browser driver. | This combination is proposed, not demonstrated. Terminal text extraction, exit-status capture, and child cleanup need a live pilot. |
| [VHS](https://github.com/charmbracelet/vhs) | Scripted terminal interaction and video generation; requires ttyd and ffmpeg. | Useful for repeatable, known CLI scenarios or demonstrations. | Its tape format describes scripted actions; an exploratory agent may be better served by a live terminal. |
| [asciinema](https://github.com/asciinema/asciinema) | Recording and replay of terminal sessions as timed terminal events. | Searchable terminal output can supplement video. | An event recording is not an MP4/WebM video and alone does not meet this request. |

The simplest initial choice is **Playwright CLI plus the existing Gimble agent harness**. Its documented shell interface avoids making MCP configuration a prerequisite. This is a recommendation based on the integration surface, not a measured performance ranking. MCP remains an option where it is already configured.

## Browser access and recording

Playwright CLI documents named sessions and `video-start`/`video-stop`, as well as separate tracing commands. Those are useful because the agent can operate the browser that is being recorded. The CLI README also distinguishes action recording, which produces Playwright code, from video recording. Do not confuse either with a saved screenshot. [Official CLI documentation](https://github.com/microsoft/playwright-cli), [local snapshot](checked-sources/playwright-cli-README.md).

Playwright's lower-level browser API is another option if the CLI proves insufficient. Its video documentation requires closing the browser context to ensure recordings are saved. A trace provides diagnostic state and actions; it is a different artifact from a playable video. [Video lifecycle](https://playwright.dev/docs/videos), [trace viewer](https://playwright.dev/docs/trace-viewer).

The workflow should explicitly stop recording and close its owned browser session before normal teardown. A failure path should do the same with a bounded cleanup opportunity. Abrupt process death can prevent finalization; a shorter scenario timeout helps with expected timeouts but does not guarantee cleanup on arbitrary cancellation. Missing or unreadable video must be reported, not presented as successful recording. No integration in this research proves recording under failure or cancellation.

At task time, `agy mcp list` reported no configured MCP servers. The repository declares Playwright in `e2e/package.json`, but this worktree had no installed `e2e/node_modules/@playwright/test`. Node, pnpm, and ffmpeg were on PATH; ttyd and asciinema were not. Installing a pinned driver/browser and demonstrating tool access from the real Gemini session belongs in the pilot.

## CLI interaction and video

For noninteractive commands, Gimble already captures exit status, stdout, stderr, and execution errors through `RunCommand`; `Check` stores that evidence in scope context. Interactive terminal behavior also needs a real terminal, rather than only piped output. [Current command implementation](../../../command.go).

A small candidate is to run a real terminal under ttyd on loopback, let the agent type and inspect it through the browser, and use the browser recorder to capture the interaction. That reuses an existing terminal implementation. It does not require a new PTY API or terminal renderer in Gimble. The initial experiment must establish how the validator obtains trustworthy terminal text and the target command's exit code: the ttyd server's own logs/status are not automatically the target's stdout or exit status.

The terminal's process lifecycle also needs demonstration. Gimble's `Service` owns descendants that remain in its process group; a PTY/session leader may establish a different group. Therefore a blanket claim that Service automatically kills every ttyd descendant would be unjustified. Confirm terminal disconnect/shutdown behavior and check for surviving owned processes during the pilot. [Service contract](../../../service.go), [ttyd options](checked-sources/ttyd-README.md).

If this path proves awkward, VHS is a reasonable option for fixed CLI scenarios. There is no need to choose several terminal implementations in advance. A timed terminal event recording may supplement either route, but the deliverable still includes actual video.

## Features and trustworthy results

The input should describe features in plain language: identity, frontend, setup, interaction, and observable successful behavior. JSON and YAML can map to the same Go data. Cucumber's readable behavior examples are useful inspiration, but its step-binding machinery is unnecessary for this first workflow. [Gherkin reference](https://cucumber.io/docs/gherkin/reference).

For known UI behavior, Playwright's retrying assertions are useful evidence: they wait for the expected state instead of checking only one instant. For CLI behavior, record the actual exit status and output. A passed command alone does not establish that the feature worked; an agent should inspect whether the observations meet the feature definition, consistent with Gimble's definition of done. [Playwright assertions](https://playwright.dev/docs/test-assertions), [Gimble validation policy](../../../docs/definition-of-done.md).

Give every declared feature an outcome, including those that could not be attempted. Distinguish a product failure from an unavailable provider or missing tool, while leaving the overall validation incomplete if required features remain unverified. Keep the original failures visible. Avoid invented accuracy scores, unmeasured token-cost comparisons, automatic assertion weakening, or a repair loop that keeps running until green.

## Startup, daily execution, and the small design

Gimble already supplies the main orchestration pieces:

- `RunCommand` for a supplied preparation/build command and finite CLI checks.
- `Service` for a foreground process whose lifetime belongs to a scope. Startup is not readiness; check readiness separately. Unexpected exit fails its scope, including exit zero.
- `Iterate` for a known list of features, each with its own scope.
- `Generate` for the agent's interaction and assessment, with existing run observation and control.

A target can be launched by the workflow or supplied as an existing URL/executable. Cleanup applies to resources the workflow owns. Keep the observing runner separate from the target when validating Gimble's own restart and cancellation behavior.

A daily scheduler can invoke the same workflow command after resolving and building latest main in an isolated checkout. Record the commit and tested executable, collect per-feature results and video paths in a caller-selected output directory, and make incomplete coverage visible. Scheduling, a feature-file runner, and video presentation are proposed additions; this research does not establish them as existing Gimble features.

The useful first experiment is two features—one browser, one CLI—with an intentional failure and cancellation. It should establish actual agent access, successful assertions, playable recordings, and cleanup. Expand that proven path to the complete feature inventory before describing the daily run as total product validation.
