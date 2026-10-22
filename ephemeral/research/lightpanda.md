# Lightpanda and Gimbal's browser tests

Researched 2026-10-21. Scope: official Lightpanda 1.0.1 and
1.1.0-nightly.10865+dba88f2d3, Playwright 1.64.0, Vitest 5.0.1,
and Gimbal's Svelte component-test harness.

## What we learned

The follow-up [capability and coverage assessment](lightpanda-capabilities.md)
separates browser gaps from Vitest integration and records controlled tests
with official binaries. Seven unchanged Gimbal component tests passed on
official nightly with an adapted harness. That narrows the adoption question
to specific coverage losses and integration costs; this is not a general
inability to test application behavior. Chromium remains configured while
that decision is explored. The assessment also records a controlled Bob
performance comparison: a compatible DOM/form cycle was about 2.5 times faster
with 6.7 times less peak browser memory on official nightly.

**Keep Chromium and run source checks on Bob.** Official Lightpanda did not
qualify as a replacement for this suite. Basic component execution and several
interactions work, but ordinary functional tests exposed browser compatibility
gaps beyond fixed SVG dimensions.

An experimental custom Lightpanda build eventually passed all 140 browser
tests after native browser patches, a custom Vitest harness, and test changes.
That result does **not** establish compatibility of an official Lightpanda
release. Building and maintaining a modified browser was outside the intended
scope. Tyler rejected that approach; [PR #652](https://github.com/tylergannon/gimbal/pull/652)
was closed and its CI compilation run was cancelled. None of that integration,
browser patching, or test changes was merged. The source-check job remains on
Bob with Chromium.

The unchanged main-branch frontend was then checked inside Bob's
`gimbal-actions-runner-1` container: the normal Playwright Chromium installation
succeeded, and `vp run test` passed all 228 tests in 48 files in 12.96 seconds.
The original Chromium download failure was not reproduced on Bob.

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

Module-mocking tests encountered a deadlock: Lightpanda's synchronous V8 module
linking defers JavaScript/WebSocket callbacks while waiting for a module fetch,
but Vitest's lazy mock interception needs a browser WebSocket round trip to
discover exports. The experimental harness changed mock-resolution timing to
work around that dependency. Neither this nor pointer timeouts establishes a
universal lack of DOM or form support.

The initial macOS attempt and the follow-up on Bob used the same released
Lightpanda version. Bob's check ran on its Arch Linux x86_64 host with Node
24.22.1, outside the registered Actions runner containers. It does not qualify
the complete suite or a CI deployment. No application component was changed.
Experimental test and harness edits remained on the closed migration branch.

## Follow-up compatibility findings

Nightly includes [PR #4016](https://github.com/lightpanda-io/browser/pull/4016),
which routes native mouse input into iframe documents. With explicit iframe
dimensions, the minimal iframe form and cancellation disclosure work. This is
an improvement over released 1.0.1, but it does not resolve every interaction.

Follow-up also examined upstream source at `9c97385c277a29138ccd69f66e3ed4bbd03a869e`
before modifying the affected browser behaviors. Observed gaps included:

- CDP scrolling scrolls the target's document but can leave it clipped by an
  ancestor iframe. Document extent also omits iframe bounds in the relevant
  case. Inline answer submission exposed this with an ordinary locator click.
- At viewport width 1000, `matchMedia('(width < 1100px)').matches` is false,
  while `(max-width: 1099px)` is true. Gimbal's responsive sidebar uses the
  range form, so this changes functional dismissal behavior.
- CDP frame metadata omits iframe names, breaking Vitest keyboard commands
  that look up `vitest-iframe` by name.
- Pointer-capture methods are absent. Clicking the splitter before testing
  its keyboard controls raises `setPointerCapture`/`hasPointerCapture` errors.

The attempted browser patches addressed ancestor scrolling, iframe extent,
and media-query ranges. They required Lightpanda's Zig toolchain and its Rust
dependencies. Compiling those during CI was the wrong deployment approach and
maintaining those patches was the wrong scope for this migration. Installing
an official binary alone would not supply those missing behaviors.

The official builds do not run the whole current suite as a drop-in
replacement. Whether a narrower Lightpanda test lane is worthwhile depends
on the coverage and adapter tradeoffs in the follow-up assessment. This
research has not changed CI.
