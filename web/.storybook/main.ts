import type { StorybookConfig } from "@storybook/sveltekit";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(js|ts|svelte)"],
  addons: ["@storybook/addon-svelte-csf", "@storybook/addon-themes"],
  framework: "@storybook/sveltekit",
};
export default config;
