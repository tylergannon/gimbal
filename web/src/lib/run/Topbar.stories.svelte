<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Topbar from "./Topbar.svelte";
  import { implementInteractiveFixture, mismatchedHistoryFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimbal/Run/Topbar",
    component: Topbar,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story
  name="Live"
  args={{
    run: implementInteractiveFixture.snapshot.run,
    connection: "live",
    activeTurn: implementInteractiveFixture.snapshot.turns["coding.1/turn.3"],
    onstop: () => undefined,
    oncancel: () => undefined,
  }}
/>

<Story
  name="Disconnected"
  args={{
    run: implementInteractiveFixture.snapshot.run,
    connection: "disconnected",
    activeTurn: implementInteractiveFixture.snapshot.turns["coding.1/turn.3"],
  }}
/>

<Story
  name="Recorded"
  args={{ run: mismatchedHistoryFixture.snapshot.run, connection: "disconnected" }}
/>
