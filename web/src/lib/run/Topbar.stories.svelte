<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Topbar from "./Topbar.svelte";
  import { implementInterviewFixture, mismatchedHistoryFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimbal/Run/Topbar",
    component: Topbar,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story
  name="Live"
  args={{
    run: implementInterviewFixture.snapshot.run,
    connection: "live",
    activeTurn: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
    onstop: () => undefined,
    oncancel: () => undefined,
  }}
/>

<Story
  name="Disconnected"
  args={{
    run: implementInterviewFixture.snapshot.run,
    connection: "disconnected",
    activeTurn: implementInterviewFixture.snapshot.turns["coding.1/turn.3"],
  }}
/>

<Story
  name="Recorded"
  args={{ run: mismatchedHistoryFixture.snapshot.run, connection: "disconnected" }}
/>
