<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Map from "./Map.svelte";
  import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";

  const { Story } = defineMeta({
    title: "Gimble/Run/Map",
    component: Map,
    parameters: { layout: "fullscreen" },
  });
</script>

<Story name="Implement interview" args={implementInterviewFixture} />
<Story name="Plan trip" args={planTripFixture} />
