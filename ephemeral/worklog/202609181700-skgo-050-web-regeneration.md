# skgo 0.5.0 web regeneration

decision: Treat the skgo 0.5.0 generated `web/` directory as the new base, then restore Gimbal-owned routes, components, remotes, and tests onto it; do not incrementally retrofit Storybook and Vitest into the old scaffold.

decision: Preserve the official Svelte compiler and Svelte-aware checker compatibility split when reconciling dependencies; the generated scaffold supplies structure, Vitest, and Storybook, while existing Gimbal behavior remains the invariant.

friction: skgo 0.5.0's minimal official Svelte scaffold has no Tailwind dependency, so shadcn-svelte 1.7.0 refuses to initialize even with the documented preset -> install Tailwind v4 and its Vite plugin, then run shadcn init.

correction: Tyler clarified that Tailwind must be added through the official `vp dlx sv add` CLI rather than direct package and Vite config edits; the missing generator integration is being filed upstream.

friction: `vp dlx -- sv add tailwindcss=plugins:none` fails on skgo 0.5.0's generated `plugins: lazyPlugins(...)` with `Expected an ArrayExpression but got CallExpression` -> temporarily expose a literal plugin array to the official add-on, then restore the generated lazy wrapper.

correction: Tyler wants the Vite+ formatter installed, working, and its formatting changes retained across the restored web application; do not minimize the diff by restoring pre-generator formatting.

decision: `just build` runs `vp fmt` immediately after `go generate`; otherwise skgo rewrites seven generated TS/JSON files into a state that fails the formatter check the build is meant to preserve.
