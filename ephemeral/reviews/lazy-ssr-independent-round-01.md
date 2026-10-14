# PR 316 independent review

Target: SKGO PR 316, f37a436d4d02993a649272e09ecd28dc3b79e4fe, in `/Users/tyler/.codex/worktrees/skgo-lazy-dev-ssr`. Implementation tree remained unchanged. Review scope is the lazy dev SSR change and its production loader interface under SKGO's pinned Kit mirroring contract.

## Evidence inspected

Read the complete four-file PR diff, surrounding renderer/module invalidation and handler fixture code, and installed Kit 3.0.0's `exports/vite/dev/generate_manifest.js`, `runtime/server/page/index.js`, `render.js`, and `runtime/error-chain.js`. Kit supplies lazy component functions and loads the requested branch and its error chain. The new loader shape matches that general contract.

Ran `TestDevSSRLoadsOnlyRequestedNodes` and `TestNativeTypedCallerForms`: pass in 4.508 seconds. The new test observed 129 startup module requests and 31 additional first-page requests, with literal source/universal-load results, unvisited failure isolation, repair, visited edits, and node-table membership edits. Ran `TestEveryRouteRendersThroughTheDevHandler` and `TestADependencyEditReachesTheNextDevDocumentWhileTheImporterIsUnchanged`: pass in 8.996 seconds, including expected error-boundary routes. No skipped tests.

An isolated copied adapter restored eager imports behind the new loader interface, injected only into TestMain's temporary web fixture through a Go test overlay. The new test failed at `startup fetched unrelated page /src/routes/about/+page.svelte`, demonstrating that its startup isolation assertion is load-bearing. Logs and mutant are in `/tmp/skgo-lazy-independent-eager.log` and `/tmp/skgo-lazy-independent-eager/`.

## Issue: failed error-component imports break valid pages

`internal/adapter/skgo-adapter/entry.js:230–232` awaits all error-component imports without isolating their rejection. Kit deliberately catches each failed error-component import and supplies `undefined` (`node_modules/@sveltejs/kit/src/runtime/error-chain.js:27`, called by `runtime/server/page/index.js:373–375`). Thus a broken optional error page must not by itself prevent a normal branch from rendering.

Reproduction: extend the existing fixture after its successful `/about` render, write root `+error.svelte` as `<script>let broken = ;</script><h1>BROKEN_ERROR_PAGE</h1>`, and request `/about` again. The current implementation returns 500 instead of the independently supplied `FIRST_ROUTE_41` document with status 200. The log identifies failure transforming `/src/routes/+error.svelte`. Temporary overlay and exact command inputs are `/tmp/skgo-lazy-error-boundary-probe/`; output is `/tmp/skgo-lazy-error-boundary-probe.log`.

This was not a working behavior of the previous eager implementation either; that implementation imported broken routes even earlier. It is nevertheless a concrete missed constraint in the lazy Kit contract being introduced, not a timing or benchmark objection. Mirror Kit's error-loader failure behavior and add a normal-page/error-page repair regression. Do not suppress errors from the actual requested component or its universal load.

Outcome: **material findings remain**.
