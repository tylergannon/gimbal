# What adopting Lightpanda would change about our tests

Investigated 2026-10-21. This is a coverage assessment, not a migration.
Chromium remains configured. No browser source was changed or compiled for
this investigation, and no application component or test assertion was edited.

## Main finding

Official Lightpanda can test useful Gimbal behavior. Seven unchanged component
tests passed on official nightly after adapting Vitest's infrastructure. The
decision is therefore about the coverage and integration costs below, rather
than a universal inability to test forms, drafts, or keyboard interactions.

There are three separate questions:

1. Does a browser implement the application API or behavior being exercised?
2. Can our current Vitest provider drive it correctly?
3. Does a passing assertion establish the same thing as it does in Chromium?

Some failures belong entirely to the provider. Others reflect a missing API.
Layout approximations can also let a test pass while proving less than its
Chromium equivalent. Counting failures alone obscures those distinctions.

## Controlled comparison

Identical small pages ran inside `gimbal-actions-runner-1` on Bob, using
Playwright 1.64.0 with these browsers:

- Chromium 156.0.8078.4, installed by Playwright.
- Official Lightpanda stable 1.0.1, the latest release checked on this date.
- Official Lightpanda nightly 1.1.0-nightly.10865+dba88f2d3.

The stable binary's SHA-256 was
`551747e35c51ba107b5d5277eca4b48e077b2a3b40f09a047bf6670448bc35fd`;
nightly was `13d04ee017d8c9412819a58ae64881e05f7a882e15d70b2e538b96104a305872`.
The binaries were copied into a research cache and executed directly. They were
not installed into the runner image or substituted into Actions.

Lightpanda was connected through `chromium.connectOverCDP`, with iframe and
stylesheet loading enabled. Browser viewport was 1000 by 600. Framed examples
also gave the iframe explicit HTML width/height attributes. No forced clicks,
pointer-capture polyfills, or native browser patches were used.

| Standalone case | Stable 1.0.1 | Official nightly | Chromium control |
| --- | --- | --- | --- |
| Fill input, press keyboard keys, collect FormData, submit through fetch, show response | Pass | Pass | Pass |
| Same form inside an in-view iframe | Click intercepted by parent BODY | Pass | Pass |
| Ordinary click on a below-fold top-level form | Outside viewport | Pass | Pass |
| Same below-fold form inside an iframe | Outside viewport | Outside viewport | Pass |
| In-view form inside an iframe placed below the parent viewport | Click intercepted | Outside viewport | Pass |
| CSS `display:none`, visible button, disabled button | Correct in sample | Correct in sample | Correct |
| Explicit 100px scroll box, 1000px content, set scrollTop to 500 | Correct in sample | Correct in sample | Correct |
| `(width < 1100px)` at width 1000 | Incorrectly false | Incorrectly false | True |
| `(max-width: 1099px)` at width 1000 | True | True | True |
| Pointer capture during pointerdown/up | Methods missing; calls throw | Methods missing; calls throw | Capture succeeds |
| Plain ES module import | Pass | Pass | Pass |
| Dynamic module response requiring a browser WebSocket callback | Pass | Pass | Pass |
| Static dependency response requiring that same callback | Callback delayed past response deadline | Callback delayed past response deadline | Pass |
| Playwright network interception of a form's API request | Pass | Pass | Pass |
| Iframe name exposed through Playwright frame metadata | Empty | Empty | `vitest-iframe` |
| Two simultaneous contexts on one CDP connection | Rejected | Rejected | Supported |
| A second CDP connection with another active context | Works in sample | Works in sample | Not needed for control |
| New context after closing the previous context | Works | Works | Works |

Failures in the table are observations in these examples, not claims that all
uses of an API fail. In particular, nightly handles the ordinary in-view iframe
case and top-level scrolling that stable does not.

## Capability and coverage implications

### Forms, messages, drafts, and ordinary keyboard input

These are viable test targets. The standalone form typed `AdaX`, submitted
FormData through fetch, and displayed the server response. Network interception
also substituted a response successfully in all three browsers.

More directly, the following existing Gimbal tests passed on official nightly:

- Five tests in [ProjectWorkspace.svelte.test.ts](../../web/src/lib/workspace/ProjectWorkspace.svelte.test.ts): ready/running drafts survive closing and reopening a view; ended sessions remain read-only; ending a session requires confirmation; rejected sends preserve the proper draft.
- The retained-question submission test in [RunQuestions.svelte.test.ts](../../web/src/lib/run/RunQuestions.svelte.test.ts).
- Cancellation disclosure in [CancellationStatus.svelte.test.ts](../../web/src/lib/run/CancellationStatus.svelte.test.ts).

All seven passed in 3.67 seconds in the same container. Those three test files
and application components were unchanged from main at `b8b8b6de`. Only the
research configuration/provider and Vitest setup changed. This qualifies those
tests with that adapter; it does not qualify the whole suite or the stock
Vitest configuration. The adapter is the experimental provider recorded in
closed [PR #652](https://github.com/tylergannon/gimbal/pull/652), without any of
that PR's browser patches or test edits.

### Layout, clipping, and scrolling

Lightpanda's documentation explicitly describes approximate sizes and synthetic
positions, with no real rendering pipeline. The comparison makes this concrete:

- An explicitly 14-by-14 SVG measured 14 by 14 in Chromium and 5 by 5 in both
  Lightpanda binaries.
- Two equal flex children in a 400px-wide row appeared at x=8 and x=208, with
  the same y coordinate, in Chromium. Nightly estimated their widths as 200px
  each but placed them at x=0 and x=5 with different y coordinates. Stable
  estimated both as 5px wide.
- Explicit overflow dimensions behaved correctly in the small scroll-box
  example. Text wrapping produced different content heights between browsers.

Consequently, adopting Lightpanda for these checks would give up confidence in
the actual arrangement, clipping, and reachability of the controls. This is
broader than screenshots or the exact size of an icon.

For Gimbal specifically:

- Dropping the 14px SVG and minimum 30px tab-height assertions gives up small
  styling guarantees. Icon presence, type, label, selection, and close behavior
  can still be checked.
- [WorkspaceShell.svelte.test.ts](../../web/src/lib/shell/WorkspaceShell.svelte.test.ts)
  checks that a long filename is not clipped and that a narrow viewport leaves
  the views at full width. Those checks need meaningful layout measurements.
- [ConversationView.svelte.test.ts](../../web/src/lib/conversation/ConversationView.svelte.test.ts)
  checks that opening Find does not move the composer. Synthetic positions do
  not establish the physical placement of that composer.
- [SessionPage.svelte.test.ts](../../web/src/lib/run/detail/SessionPage.svelte.test.ts)
  tests following new output while preserving an earlier reader's position.
  Explicit overflow fixtures can exercise the follow/retain policy, but they
  cannot establish that real text wrapping and flex layout produce the right
  viewport. Both are useful claims; they have different proof requirements.

There is also a narrower iframe automation problem: nightly clicked the long
top-level form but could not click the same form inside an iframe. Vitest
introduces that iframe even when the application feature under test does not
use one. A top-level application test may avoid this particular barrier; that
is a change in testing approach and remains unqualified for the full app.
Using a forced or programmatic click would cease checking ordinary pointer
reachability, so it should not be reported as equivalent evidence.

Source: [Lightpanda architecture](https://lightpanda.io/docs/core-concepts/architecture-overview).

### Responsive sidebar behavior

This is an API compatibility gap, not a demand for an exact pixel size.
[WorkspaceShell.svelte](../../web/src/lib/shell/WorkspaceShell.svelte:35) uses
`new MediaQuery('width < 1100px', false)` to decide whether clicking outside
the sidebar should dismiss it. Both official binaries returned false for the
corresponding range query at width 1000, while Chromium returned true.

The older max-width syntax works in the sample. These results do not establish
that every media query fails. They do establish that this application's current
responsive dismissal branch is not exercised correctly by Lightpanda.

Stubbing the media-query result could test our branch logic, while giving up
verification that the real browser query selects that branch. Changing the
application's query syntax would be a product-code decision; it was not done
as part of this research.

The unmodified source also explicitly tests range syntax as unsupported:
[MediaQuery.zig at the examined revision](https://github.com/lightpanda-io/browser/blob/9c97385c277a29138ccd69f66e3ed4bbd03a869e/src/browser/css/MediaQuery.zig#L623).

### Pointer dragging versus keyboard resizing

Both official binaries lack `setPointerCapture`, `hasPointerCapture`, and
`releasePointerCapture`. A real pointerdown/up on the standalone handle called
those methods and threw. Chromium successfully acquired capture.

[Splitter.svelte](../../web/src/lib/run/Splitter.svelte:30) uses these methods
to keep receiving drag events when the pointer leaves the handle. Its current
tests are two keyboard-resizing tests that first click the handle. That setup
incidentally invokes the unsupported drag code.

Focusing the handle directly would let a keyboard-only test retain its stated
purpose. It would not prove pointer capture or mouse dragging. The current
suite already lacks a complete drag-outside-the-handle movement test, so that
coverage should not be presented as something it currently proves well.

### Accessibility names

The same flex button with a status image, title, and unseen image produced:

- Chromium: `Failed Review Unseen changes`.
- Both Lightpanda binaries: `FailedReviewUnseen changes`.

The individual image labels and text were present in both. Several Gimbal
locators concatenate these parts and therefore fail because of the missing
separators. Checking the parts separately can preserve status/title/unseen
assertions. It gives up fidelity for the complete computed accessible name;
that can matter to accessibility and should not be dismissed as only a
selector nuisance.

### Vitest integration: a migration cost, not lost application behavior

Twenty-one of our 31 browser test files use `vi.mock` or `vi.doMock`. The mock
loading problem therefore affects a large part of the current test structure.

The standalone reproduction removed Vitest: an ES module imported a static
dependency whose HTTP response waited for a browser WebSocket callback. Both
Lightpanda builds deferred that callback until the server supplied a timeout
response after two seconds. Chromium completed the exchange immediately. A
direct dynamic import requiring the same callback succeeded in all browsers.
Plain imports and ordinary API-response interception succeeded too.

This isolates a dependency cycle, not a general failure of JavaScript, modules,
WebSockets, or mocking. It matches the browser source's synchronous import wait
that defers other callbacks:
[ScriptManagerBase.zig](https://github.com/lightpanda-io/browser/blob/9c97385c277a29138ccd69f66e3ed4bbd03a869e/src/browser/ScriptManagerBase.zig#L304).

The experimental adapter resolves mock factories before importing their
consumers. That enabled the seven unchanged component tests above, including
async factories. It depends on Vitest internals and factory ordering. It is
maintenance we would take on; it is not a verified universal solution for
arbitrary mock graphs. Network-response interception is another demonstrated
primitive, but migrating the current component suite to it has not been done.

Two additional adapter issues do not require discarding keyboard or isolation
tests:

- The DOM iframe really has a name, but CDP frame metadata reports it as empty.
  Direct locator-based keyboard input inside the iframe works. A provider can
  find the frame from its DOM element instead of relying on its reported name.
- A second simultaneous context on one connection is rejected. A second CDP
  connection successfully created another context while the first remained
  active, and creating a context after closing the previous one also worked.
  Sequential execution is one option; per-worker connections are a plausible
  route to parallelism. Vitest parallel execution through that route has not
  been implemented or qualified here.

## Decisions the evidence supports

| Test intent | What an official-Lightpanda lane can establish | What would remain unproven |
| --- | --- | --- |
| Draft, session, answer, and request state | Demonstrated for seven unchanged tests with adapter | Whole-suite compatibility |
| Keyboard controls | Direct input works; component keyboard policy remains testable | Pointer dragging/capture |
| Hidden/disabled controls | Basic CSS visibility and disabled state | Human-visible placement and occlusion in general |
| Scroll-following policy | Deterministic overflow and state transitions can be exercised | Real content geometry, clipping, and some iframe reachability |
| Responsive dismissal | Branch logic could be isolated | Current range-query integration with the browser |
| Status and unseen indicators | Individual labels, state, and content | Chromium-equivalent combined accessibility name |
| Appearance and fitting content | DOM content and some approximate dimensions | Accurate layout, screenshots, icon sizing, clipping |

A split where Lightpanda handles state/DOM behavior and Chromium handles the
small set of browser-layout/API integration checks is a candidate, not an
adopted design. It preserves more confidence than removing all checks that
fail under Lightpanda. Using only Lightpanda would mean explicitly accepting
the unproven items above or changing what those tests claim.

## Performance: meaningful savings on compatible work

The earlier patched-browser suite used sequential execution while the Chromium
baseline ran files in parallel. Those times are not a controlled performance
comparison and should not be used to decide which browser is faster.

A separate small comparison ran all three official browsers in the same Bob
runner container (8 CPU quota, 16 GiB memory limit), through the same Playwright
CDP client. Each browser ran three fresh-process rounds with twenty warmed
load-and-interact cycles per round. Engine order alternated between rounds.
All 180 cycles completed successfully.

The local page fetched JSON, created 200 DOM rows, accepted keyboard input,
submitted FormData through fetch, and displayed the expected response. Each
cycle verified the row count and submitted value. It used ordinary top-level
controls, with no forced clicks, native patches, or assertions removed. This is
a compatible DOM/form workload; it does not exercise the iframe/layout failures
above or constitute a Gimbal suite benchmark.

| Metric | Chromium headless shell 156 | Lightpanda stable 1.0.1 | Official nightly |
| --- | ---: | ---: | ---: |
| New browser process through first loaded/ready page, median of 3 | 86 ms | 46 ms | 45 ms |
| Warm load, JSON fetch, DOM population and count check, median of 60 | 41 ms | 24 ms | 25 ms |
| Fill, keyboard input, click, submit and response check, median of 60 | 76 ms | 22 ms | 22 ms |
| Combined load/interact cycle, median of 60 | 117 ms | 47 ms | 47 ms |
| Peak total browser memory during measured cycles | 279 MiB | 41 MiB | 42 MiB |
| Browser processes counted at peak process count | 6 | 1 | 1 |

For nightly that is about 2.5 times faster per completed cycle and 6.7 times
less peak memory (approximately 85% lower). Stable gave essentially the same
performance on this compatible example. The three rounds' peak memory was
275–279 MiB for Chromium and 41–42 MiB for nightly. Warm load p95 was 44 ms
for Chromium and 27 ms for nightly; interaction p95 was 77 ms and 24 ms.

Memory was sampled every 100 ms from Linux `smaps_rollup`, summing proportional
set size (PSS) across each browser's process tree. PSS avoids counting shared
pages in full in every Chromium child. The Node driver and local HTTP server
were excluded for both browsers. These are short-run sampled peaks, not
long-running capacity or leak measurements. Fresh-process startup used warm
filesystem caches; it is not a cold-machine installation benchmark. Chromium
was Playwright's smaller headless-shell build, with sandbox disabled inside
the container. Both browsers connected over CDP. Lightpanda loaded iframe and
stylesheet resources, though the sample needed no iframe or external assets.
CPU consumption was not measured in this local comparison.

Lightpanda's [published benchmark](https://lightpanda.io/docs/core-concepts/benchmarks)
reports a larger advantage for 100 local page-load/extraction iterations:
16 ms versus 185 ms average, with peak PSS of 21.2 MB versus 402.1 MB, about
11 times faster and 19 times less memory. Its 933-page crawl at 25-way
concurrency reports 4.81 seconds and 123 MB versus 46.70 seconds and 2 GB.
Those are vendor measurements on different workloads and browser revisions,
not expected Gimbal CI speedups. The linked
[methodology and source](https://github.com/lightpanda-io/demo/blob/main/BENCHMARKS.md)
identify an older Lightpanda commit and Chrome 143.

The architecture explains why the advantage is plausible: Lightpanda avoids a
conventional rendering pipeline. That saves work but also underlies some of
the layout/interaction limitations documented here. Network waits, Go builds,
Vite transforms, Vitest setup, test parallelism, and retries still contribute
to whole-job time. A smaller browser can increase worker capacity without
making the complete CI job 2.5 or 11 times faster.
