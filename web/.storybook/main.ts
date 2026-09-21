import { basename, resolve } from "node:path";
import { existsSync } from "node:fs";
import type { StorybookConfig } from "@storybook/sveltekit";

const mocks = resolve(import.meta.dirname, "../src/lib/storybook/mocks");

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(js|ts|svelte)"],
  addons: ["@storybook/addon-svelte-csf", "@storybook/addon-themes"],
  framework: "@storybook/sveltekit",
  // Remote functions are answered by the Go server, which Storybook does not
  // have. Every "*.remote" import resolves to its stand-in under
  // src/lib/storybook/mocks; see src/lib/storybook/remotes.ts.
  viteFinal: (config) => {
    config.plugins = [
      {
        name: "gimble-mock-remotes",
        enforce: "pre",
        resolveId(source, importer) {
          if (!/\.remote(\.[jt]s)?$/.test(source) || importer?.startsWith(mocks)) return;
          // Not named "*.remote.ts": SvelteKit refuses to serve those to a browser.
          const mock = resolve(mocks, basename(source).replace(/\.remote(\.[jt]s)?$/, ".ts"));
          if (existsSync(mock)) return mock;
          this.error(`no Storybook stand-in for ${source}: add ${mock}`);
        },
      },
      ...(config.plugins ?? []),
    ];
    return config;
  },
};
export default config;
