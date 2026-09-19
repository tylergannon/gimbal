<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import CancelGuard from "./CancelGuard.svelte";
  import { implementInterviewFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/Cancel guard",
    component: CancelGuard,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story
  name="Confirm whole-run cancellation"
  args={{
    open: true,
    run: implementInterviewFixture.snapshot.run,
    activeTurn: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
    openScopes: 3,
    supervisors: 2,
    onopenchange: () => undefined,
    onstop: () => undefined,
    oncancel: () => undefined,
  }}
/>
