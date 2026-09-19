<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import HistoryLanes from "./HistoryLanes.svelte";
  import { mismatchedHistoryFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/History lanes",
    component: HistoryLanes,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  import type { RunSelection } from "./selection.js";
  let selected = $state<RunSelection>();
</script>

<Story name="Mismatched graph" asChild>
  <div style="height: 900px; display: flex;">
    <HistoryLanes
      snapshot={mismatchedHistoryFixture.snapshot}
      reason={mismatchedHistoryFixture.graphNotice}
      {selected}
      onselect={(next) => (selected = next)}
    />
  </div>
</Story>
