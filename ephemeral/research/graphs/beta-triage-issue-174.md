# #174: Move the Astro docs site out of the repository root

https://github.com/tylergannon/gimble/issues/174

## Problem

The GitHub Pages docs site is an Astro app whose files sit in the repository root. Commit 795d860 ("Restore static Astro docs for GitHub Pages", 2026-09-11) put them there:

- `astro.config.mjs`, `svelte.config.js`, `tsconfig.json`
- `package.json` (named `gimble-docs`), `pnpm-lock.yaml`, `pnpm-workspace.yaml`
- `components.json` (a shadcn-svelte config pointing at `src/styles/global.css`)
- `src/` (pages, layouts, components, `src/lib/components/ui/button`, `src/styles/global.css`)
- `public/` (favicon, mascot)
- `.github/workflows/docs.yml`, which runs `pnpm install`, `pnpm run check`, and `pnpm run build` from the root and uploads `dist`

The repository root is the Go module `github.com/tylergannon/gimble`. The product web application is the SvelteKit app in `web/`, with its own `package.json`. So today the root carries a second, unrelated Node project, and:

- `components.json`, `src/`, `svelte.config.js`, and `tsconfig.json` at the root read as if they belong to the product app. They do not. While drafting the run page design brief today, this was mistaken for the app's own shadcn-svelte install.
- Two `pnpm-lock.yaml` and two `package.json` files in the tree, one of them at the root, with different pnpm versions pinned (`11.22.0` at the root, `11.25.0` in `web/`).
- `AGENTS.md` describes the layout as root package, `web/`, `cmd/`, `internal/`. The docs site is not in that description.

## Ask

Move the whole Astro site into one directory that says what it is. `site/` or `docs-site/` is fine; `docs/` is taken by `docs/definition-of-done.md`, which is repository documentation, not the website. Then:

- Update `.github/workflows/docs.yml` to run in that directory (`working-directory`, or `cd` in each step) and upload `<dir>/dist`.
- Leave the root with the Go module, `web/`, `e2e/`, `cmd/`, `internal/`, `docs/`, `ephemeral/`, `AGENTS.md`, `README.md`, `Justfile`, and nothing that belongs to a Node project.
- Add one line to `AGENTS.md`'s layout section naming the directory and what it is.
- Nothing in `web/` or the Go build depends on the root Node files, so `just build` and `just e2e` must be unaffected. Confirm by running both.

## Done when

- `git ls-files` at the root shows no `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `astro.config.mjs`, `svelte.config.js`, `tsconfig.json`, `components.json`, `src/`, or `public/`.
- The Pages workflow builds and deploys from the new directory on a push to main.
- `just build` and `just e2e` pass.
