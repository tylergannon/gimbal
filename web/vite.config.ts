import { workflowSource } from "./dev/source-plugin.mjs";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig, lazyPlugins } from "vite-plus";
import lightpanda from "./dev/lightpanda";
import adapter from "@skgo/sveltekit-adapter";
import { sveltekit } from "@sveltejs/kit/vite";

export default defineConfig(({ command }) => ({
  // Dependencies use this Node convention even in SKGo's neutral JS engine.
  define: {
    "process.env.NODE_ENV": JSON.stringify(
      process.env.NODE_ENV ?? (command === "serve" ? "development" : "production"),
    ),
  },
  fmt: {},
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
  plugins: lazyPlugins(() => [
    workflowSource(),
    tailwindcss(),
    sveltekit({
      compilerOptions: {
        // Force runes mode for the project, except for libraries. Can be removed in svelte 6.
        runes: ({ filename }) =>
          filename.split(/[/\\]/).includes("node_modules") ? undefined : true,
        experimental: { async: true },
      },
      adapter: adapter(),
      experimental: { remoteFunctions: true },
      paths: { origin: process.env.ORIGIN ?? "http://127.0.0.1:8080" },
    }),
  ]),
  test: {
    projects: [
      {
        extends: "./vite.config.ts",
        test: {
          name: "client",
          browser: {
            enabled: true,
            // Headless checks need the full viewport: the runner UI scales its
            // iframe and can intercept clicks on controls near its edges.
            ui: false,
            provider: lightpanda,
            orchestratorScripts: [{ src: "./dev/lightpanda-viewport.js" }],
            viewport: { width: 1440, height: 900 },
            instances: [{ browser: "chromium", headless: true }],
          },
          include: ["src/**/*.svelte.{test,spec}.{js,ts}"],
          exclude: ["src/lib/server/**"],
        },
      },

      {
        extends: "./vite.config.ts",
        test: {
          name: "server",
          environment: "node",
          include: ["src/**/*.{test,spec}.{js,ts}"],
          exclude: ["src/**/*.svelte.{test,spec}.{js,ts}"],
        },
      },
    ],
  },
}));
