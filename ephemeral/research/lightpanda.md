# Lightpanda and Gimbal's browser tests

Researched 2026-10-21. Scope: Lightpanda 1.0.1, Playwright 1.64.0,
Vitest 5.0.1, and Gimbal's current Svelte component-test harness.

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

Module-mocking tests also encountered a browser disconnect while resolving a
manual mock. That failure's cause has not been isolated. Neither disconnects
nor pointer timeouts should be described as a universal lack of DOM or form
support.

The initial macOS attempt and the follow-up on Bob used the same released
Lightpanda version. Bob's check ran on its Arch Linux x86_64 host with Node
24.22.1, outside the registered Actions runner containers. It does not qualify
the complete suite or a CI deployment. No browser tests or application code were
changed as part of this research.

## Questions still open

- Which existing interaction tests run unchanged with the CDP provider?
- Does a current nightly address the observed hit testing, or is an upstream
  issue needed? Only the released 1.0.1 binary has been evaluated.
- What is the smallest maintained provider/lifecycle integration for Gimbal?
- Which styling assertions should be removed on their own merits? This should
  not become a second rendering suite merely to preserve fixed pixel values.

The research supports further Lightpanda integration work. It does not yet
establish either complete suite compatibility or that Lightpanda is unusable.
