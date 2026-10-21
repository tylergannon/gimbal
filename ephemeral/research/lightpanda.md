# Lightpanda and Gimbal's browser tests

Researched 2026-10-21. Scope: Lightpanda 1.0.1, nightly
1.1.0-nightly.10865+dba88f2d3, a local build of upstream commit
9c97385c277a29138ccd69f66e3ed4bbd03a869e with two scrolling fixes
and media-query range support,
Playwright 1.64.0, Vitest 5.0.1, and Gimbal's Svelte component-test harness.

## What we learned

Lightpanda can run Gimbal Svelte components through Playwright and Vitest.
The first failed attempt omitted iframe loading, which Vitest needs. That
failure did not establish that the integration was unsupported. The remaining
question is compatibility with the interactions our tests exercise, rather
than whether an SVG has exactly the expected pixel dimensions.

## Browser model

Lightpanda is an independent browser implemented in Zig, using V8 for JavaScript
and native DOM and Web API implementations. It exposes CDP to automation clients.
It does not implement a conventional visual rendering pipeline. Its architecture
document describes element sizes as approximations and positions as simulated
from document structure. Screenshot output does not reproduce the page's actual
visual layout. This is a specific limitation for testing appearance, not a reason
to dismiss functional browser automation.

Source: [architecture overview](https://lightpanda.io/docs/core-concepts/architecture-overview).

It implements CSS processing, computed styles, visibility, event handling,
forms, network interception, storage, and frames. These features have partial
web-platform coverage; the existence of an API does not guarantee Chromium's
behavior in every case. Version 1.0.1 added or improved form submission,
computed styles, hit testing, iframe behavior, and keyboard input inside frames.

Source: [1.0.1 release notes](https://github.com/lightpanda-io/browser/releases/tag/1.0.1).

## Connection and resource loading

Playwright connects with `chromium.connectOverCDP`, rather than launching a
Chromium executable. Here `chromium` names Playwright's protocol driver; the
browser process can be Lightpanda. No Chromium download is necessary for this
connection.

Source: [Lightpanda's Playwright instructions](https://lightpanda.io/docs/usage/cdp/playwright).

In `serve` mode, iframe, stylesheet, image, and worker loading are opt-in.
For this harness, start the server with explicit `--load-resources iframe`,
`--load-resources stylesheet`, and `--load-resources worker` options. Each is a
separate repeated option. Vitest executes components in an iframe; omitting
iframe loading prevents those tests from starting. Loading external stylesheets
also matters to CSS-dependent visibility and interaction checks. Worker loading
was enabled during investigation; it has not been established as necessary for
these particular components.

The shipped 1.0.1 `serve --help` agrees with these options. Do not infer serve
mode's defaults from MCP or agent mode, whose stylesheet defaults differ.

Source: [common CLI options](https://lightpanda.io/docs/reference/cli/common-options).

## Vitest integration

Gimbal uses Vitest Browser Mode through `vite-plus/test/browser-playwright`,
not the Playwright Test runner. The provider's built-in `connectOptions` uses
Playwright's `connect` protocol, whereas Lightpanda exposes CDP. Simply changing
`browser: "chromium"` or supplying Lightpanda's executable path is not the
integration used here.

The experiment used the exported `PlaywrightBrowserProvider` with a browser
already connected over CDP, preserving Vitest's test runner and locator commands.
That demonstrated a working path for a basic component. It remains an
experimental provider configuration, not an upstream-documented Lightpanda
integration for Vitest.

Sources: [Vitest Playwright configuration](https://vitest.dev/config/browser/playwright),
Gimbal's [configuration](../../web/vite.config.ts), and the installed
`@vitest/browser-playwright` 5.0.1 implementation and declarations.

Upstream [issue #3076](https://github.com/lightpanda-io/browser/issues/3076)
requests better Playwright Test compatibility. It is open as of this research.
It concerns a different runner and is evidence of integration work in progress,
not proof that our Vitest suite cannot run.

## Implications for Gimbal

A fixed 14-by-14 SVG expectation is an implementation-detail assertion, not a
prerequisite for functional testing. Other current checks include tab height,
viewport width, computed opacity, clipping, and scroll positions. Their value
should be judged against the behavior they protect; preserving them unchanged
was not a requirement from Tyler.

With resource loading configured, basic component execution works. Workspace
and cancellation-disclosure interactions exposed pointer hit-testing problems:
Playwright found the target but reported that surrounding HTML or BODY
intercepted the click. That occurred before the SVG-size assertion. A minimal
plain HTML form also submitted successfully at the top level but failed at the
button click when the same form was inside an iframe. This gives a smaller
reproduction independent of Svelte and the fixed-size assertions.

Neither disconnects nor pointer timeouts establish a universal lack of DOM or
form support. Follow-up investigation identified concrete causes below.

The initial macOS attempt and the follow-up on Bob used the same released
Lightpanda version. Bob's check ran on its Arch Linux x86_64 host with Node
24.22.1, outside the registered Actions runner containers. It does not qualify
the complete suite or a CI deployment. The initial research changed no tests.
The subsequent integration branch changes the harness and selected test
assertions; it has not changed application components.

## Follow-up integration findings

### Native iframe input and scrolling

Nightly includes upstream [PR #4016](https://github.com/lightpanda-io/browser/pull/4016),
which routes native mouse input into iframe documents. With explicit HTML
iframe dimensions, the minimal iframe form and Gimbal's cancellation disclosure
work with normal Playwright clicks. Released 1.0.1 fails the same interaction.

Vitest sizes its iframe using CSS custom properties. Copying its configured
viewport into the iframe's HTML width and height attributes gives Lightpanda
the intended test viewport, including viewport changes.

Current upstream CDP scrolling only scrolls the target's own frame. A control
can therefore be inside that frame's viewport but clipped by an ancestor frame.
Additionally, document extent excludes the synthetic bottom of an iframe whose
height comes from an HTML attribute. A local native patch scrolls ancestor
frames when needed and includes iframe viewport bounds in document extent.
The patch adds a CDP regression test. Lightpanda's full suite passed 1968 tests
with 7 skips; the attribute-height regression also passed separately.

With this research build, all 22 existing run-page tests passed, including
ordinary form filling and inline answer submission. This is evidence for the
patched source build; it must not be attributed to the official nightly.

### Module loading and mocks

Lightpanda's `ScriptManagerBase.waitForImport` runs during synchronous V8 module
instantiation. It delivers only the required module transfer while linking and
defers callbacks that could reenter V8. Vitest's lazy manual-mock interception
needs a browser WebSocket round trip to discover exports. When requested by a
static dependency during linking, each side waits for the other.

Resolving a factory during initial server registration is insufficient:
`importOriginal` waits for Vitest's pending registrations, including its own.
The working experiment resolves client factories after registration completes,
then registers them again so the server caches export keys before linking.
Factories are resolved in their declaration order; that is an integration
constraint, not a claim of complete compatibility with arbitrary mock graphs.
All five workspace tests passed with their existing async mock factories.

Svelte's async compileModule transform also inserted reactivity-tracker imports
inside hoisted mock factories. Excluding ordinary TypeScript test modules from
rune compilation removes that initialization-order problem. Application rune
modules still compile normally.

### Additional harness and assertion boundaries

- The inherited provider prewarm must be removed or it still launches Chromium.
- Lightpanda allows only one browser context per CDP connection. Tell Vitest
  that the provider executes browser files sequentially.
- CDP frame metadata omits iframe names. Resolve the actual test iframe's DOM
  element for keyboard commands instead of looking it up by frame name.
- Keep mock-registration bookkeeping scoped to its lifetime; retaining it
  after Vitest clears mocks makes later test files deadlock.
- CSS layout affects whitespace in computed accessible names. Check the
  relevant status image, title, unseen indicator, and control independently
  instead of expecting one precise CSS-dependent concatenation.
- A tail-following test needs actual overflow before an earlier-reader
  assertion is meaningful. Explicit fixture dimensions establish that premise
  without relying on approximate text or flex layout.
- Exact SVG dimensions, minimum tab pixel height, and a fixed view width were
  removed from the candidate functional suite. Keyboard selection, status,
  draft preservation, submission, and sidebar dismissal remain exercised.

Lightpanda also rejects MQ4 range comparisons: at width 1000, `(width < 1100px)`
is false while `(max-width: 1099px)` is true. Gimbal's sidebar uses the range
form. The native patch adds width and height comparisons, including reversed
operands and chained bounds, with boundary and malformed-input coverage.
The complete Lightpanda suite still passed 1968 tests with 7 skips.

The final host run passed **all 140 tests across all 31 browser files** in
32.73 seconds. No application component was changed. This includes keyboard
resizing, status and unseen indicators, narrow-viewport sidebar dismissal,
tail following and earlier-reader preservation, workspace drafts, and inline
answer submission. Keyboard-only splitter tests now focus the handle directly;
Lightpanda does not implement pointer capture, and clicking that handle had
unnecessarily exercised drag handlers before testing the keys.

## Reproducing the candidate

The complete native changes, including their regression tests, are kept in
[`scripts/ci/lightpanda.patch`](../../scripts/ci/lightpanda.patch). Bob's
[`prepare-lightpanda.sh`](../../scripts/ci/prepare-lightpanda.sh) builds the
pinned upstream revision with that patch. Its cache key includes the complete
patch hash. Zig 0.17.0 is checksum-verified and Rust 1.94.0 is selected for the
build. The static-V8 build targets Ubuntu 24.04's libc for the Actions runners.
The script exports the resulting absolute `LIGHTPANDA_BINARY` path through
`GITHUB_ENV` in Actions; the Vite provider uses it instead of launching Chromium.

From the repository root on Linux x86_64:

```sh
export LIGHTPANDA_BINARY=$(bash scripts/ci/prepare-lightpanda.sh)
cd web
vp run test
```

The CI container qualification remains in progress. These patches have not
been released by Lightpanda; a plain stable or nightly binary is not yet an
equivalent replacement. The provider also depends on Vitest 5.0.1 mocker
internals, so a runner upgrade requires rechecking that integration.
