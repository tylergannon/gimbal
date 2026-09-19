import type { Preview } from "@storybook/sveltekit";
import "../src/app.css";

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
  },
  initialGlobals: { theme: "light" },
  globalTypes: {
    theme: {
      description: "Light or dark, the app's own .dark class",
      toolbar: {
        title: "Theme",
        icon: "sun",
        items: [
          { value: "light", title: "Light", icon: "sun" },
          { value: "dark", title: "Dark", icon: "moon" },
        ],
        dynamicTitle: true,
      },
    },
  },
  // Dark mode is the app's own convention: the .dark class app.css declares as
  // the dark variant, set on the document the story renders into.
  beforeEach({ globals }) {
    document.documentElement.classList.toggle("dark", globals.theme === "dark");
  },
};

export default preview;
