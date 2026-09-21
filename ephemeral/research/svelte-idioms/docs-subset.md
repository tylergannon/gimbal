# The Svelte docs subset the reviewers read

`svelte-idiomatic.txt` (~13,600 lines, ~115k tokens; committed on purpose so cited line numbers stay fixed and the audit has a pinned copy) is a verbatim subset of https://svelte.dev/llms-full.txt, fetched 2026-09-21. The full file (~37,800 lines, ~300k tokens) overflowed a reviewer's context, so whole top-level sections were selected, never excerpted, in original order.

Regenerate: download llms-full.txt, split on top-level `# ` headings outside code fences, and keep exactly these 79 sections:

- .svelte files
- .svelte.js and .svelte.ts files
- What are runes?
- $state
- $derived
- $effect
- $props
- $bindable
- Basic markup
- {#if ...}
- {#each ...}
- {#key ...}
- {#await ...}
- {#snippet ...}
- {@render ...}
- {@html ...}
- {@attach ...}
- {@const ...}
- {let/const ...}
- bind:
- use:
- style:
- class
- await
- Scoped styles
- Global styles
- Custom properties
- Nested `<style>` elements
- `<svelte:boundary>`
- `<svelte:window>`
- `<svelte:document>`
- `<svelte:body>`
- `<svelte:head>`
- `<svelte:element>`
- `<svelte:options>`
- Stores
- Context
- Lifecycle hooks
- Hydratable data
- Best practices
- Svelte 5 migration guide
- svelte/action
- svelte/attachments
- svelte/events
- svelte/reactivity/window
- svelte/reactivity
- Project structure
- Web standards
- Routing
- Loading data
- Form actions
- Page options
- State management
- Remote functions
- Environment variables
- Advanced routing
- Hooks
- Errors
- Link options
- Server-only modules
- Snapshots
- Shallow routing
- Accessibility
- @sveltejs/kit/hooks
- $app/env
- $app/env/private
- $app/env/public
- $app/environment
- $app/forms
- $app/navigation
- $app/paths
- $app/server
- $app/state
- $app/stores
- $env/dynamic/private
- $env/dynamic/public
- $env/static/private
- $env/static/public
- $lib

Dropped: the Svelte CLI and Svelte AI parts; adapters and deployment; packaging; integrations; migration guides other than Svelte 5's; custom elements; browser support; `svelte/compiler`, `svelte/server`, `svelte/legacy`; compiler and runtime errors and warnings; animation modules and directives (`transition:`, `animate:`, `svelte/motion`, …); the `@sveltejs/kit` type reference; configuration; FAQs; testing; TypeScript.

Line numbers cited in `dos-and-donts.md` refer to this subset.
