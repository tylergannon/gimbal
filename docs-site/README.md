# Gimble documentation site

This standalone SvelteKit application builds the static documentation published
to GitHub Pages. It is separate from the live Gimble application in `../web`.

```sh
pnpm install
pnpm run check
pnpm run build
```

Set `BASE_PATH=/gimble` when building for the project Pages URL.
