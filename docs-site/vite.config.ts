import tailwindcss from "@tailwindcss/vite";
import { mdsvex } from "mdsvex";
import adapter from "@sveltejs/adapter-static";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig, lazyPlugins } from "vite-plus";

const basePath = (process.env.BASE_PATH ?? "") as "" | `/${string}`;

export default defineConfig({
  fmt: {},
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
  plugins: lazyPlugins(() => [
    tailwindcss(),
    sveltekit({
      compilerOptions: {
        // Force runes mode for the project, except for libraries. Can be removed in svelte 6.
        runes: ({ filename }) =>
          filename.split(/[/\\]/).includes("node_modules") ? undefined : true,
      },
      adapter: adapter({
        pages: "dist",
        assets: "dist",
        fallback: undefined,
        strict: true,
      }),
      paths: {
        base: basePath,
      },
      preprocess: [mdsvex({ extensions: [".svx", ".md"] })],
      extensions: [".svelte", ".svx", ".md"],
    }),
  ]),
});
