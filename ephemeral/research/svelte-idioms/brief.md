You are one of ten reviewers auditing Gimbal's web application for idiomatic Svelte 5 / SvelteKit. Every reviewer gets this exact brief. Work independently.

## The rule you are enforcing

**If it CAN be done with a Svelte or SvelteKit high-level API or paradigm, it MUST be.** Bare browser APIs, hand-rolled state machines, and framework workarounds where Svelte/SvelteKit already provides the tool are defects. The motivating example: code calling `window.history.pushState` / `replaceState`, listening for `popstate`, and parsing `window.location.search` with `URLSearchParams` to hold page state, instead of real SvelteKit routes (`/runs/[runID]/sessions/[sessionID]`), `<a href>`, `goto`, `page` from `$app/state`, layouts, shallow routing, or `snapshot`. The owner's own rules: never use query params for page state; never use the History API directly; the routing exists so you do not have to.

Other things of that kind to hunt: `document.querySelector` / `addEventListener` on window or document where `<svelte:window>`, `<svelte:document>`, `bind:this`, attachments (`{@attach}`), or actions fit; `$effect` used to derive state (should be `$derived`) or to sync state into other state; manual `setInterval`/subscriptions without the Svelte-idiomatic lifecycle; stores or classes where runes fit; manual `fetch` where a load or remote function belongs; `localStorage` handling that ignores SSR; hand-built form handling where remote `form` / `enhance` fits; `EventSource`/streams managed in ways SvelteKit has a pattern for; prop drilling of callback bundles where the component-factoring skill says ownership belongs elsewhere; `untrack` hacks; `typeof window` checks where `browser` from `$app/environment` or lifecycle placement is right; deprecated Svelte 4 syntax (`on:click`, `export let`, `$:`, `<slot>`, `createEventDispatcher`); `svelte-ignore` comments hiding real a11y or reactivity problems; code bent to fit a test harness (Storybook, vitest) instead of the framework.

## Step 1 — read everything first (required, no skipping, no summarising tools)

1. Read the ENTIRE file `/private/tmp/claude-501/-Users-tyler-src-gimbal--claude-worktrees-detail-view-redesign-6d9ff0/1461ccea-11c4-48f2-8c47-aae787bcbe43/scratchpad/svelte-idiomatic.txt` — a verbatim subset of the Svelte + SvelteKit documentation (https://svelte.dev/llms-full.txt) with CLI, AI, adapter/deployment, compiler-API, and animation sections removed; 13,562 lines. Use the Read tool in consecutive 2000-line chunks (offset 1, 2001, 4001, … through 13562) until you have read EVERY line. Do not use WebFetch, grep, or sampling as a substitute. State at the end of your report the last line number you read.
2. Read the ENTIRE skill `/Users/tyler/.claude/skills/svelte-component-factoring/SKILL.md`.

## Step 2 — review every page of the application

The application is the SvelteKit app at `/Users/tyler/src/gimbal/web` (git `main`, clean checkout). This is READ-ONLY: do not edit, create, or delete any file under `/Users/tyler/src/gimbal`, do not run git commands that change anything, do not install packages, do not start servers. Never run `find /` or scan from `/` or `$HOME`; search only inside `/Users/tyler/src/gimbal/web`.

Pages (every one is in scope): `web/src/routes/+layout.svelte`, `+error.svelte`, `+page.svelte` (runs list), `about/+page.svelte`, `conversations/+page.svelte`, `runs/[runID]/+page.svelte` — plus every component, module, and `hooks` file those pages use under `web/src/lib` and `web/src` (follow the imports), and `web/svelte.config.js` / `web/vite.config.ts`. Generated files are out of scope as code (`web/src/lib/skgo/**`, `*_gen.go`, `*.remote.ts` stubs, `+page.server.ts` stubs), but how app code USES them is in scope. Tests and stories are in scope only where the app code was bent to suit them.

Read the code itself. Do not guess about what a file says.

## Step 3 — report

Return EXACTLY ten findings, most severe first. Each finding:

- **Where:** `path:line` (repo-relative, e.g. `web/src/routes/runs/[runID]/+page.svelte:230`), plus any other sites with the same defect.
- **What it does now:** one or two sentences, quoting the offending code briefly.
- **The idiomatic replacement:** the specific Svelte/SvelteKit API or paradigm, and how it would be used here.
- **Doc basis:** the section heading in the docs file (and its line number there) that establishes the idiom.
- **Severity:** high (bare browser API / framework bypass / wrong data flow), medium (non-idiomatic but contained), low (style).

No preamble, no praise, no summary of the app. Findings must be real and verifiable at the cited line; do not pad to reach ten with trivia if a more serious defect exists anywhere in the app. Finish with one line: `Docs read through line N of 13562.`

Hard time cap: 45 minutes.
