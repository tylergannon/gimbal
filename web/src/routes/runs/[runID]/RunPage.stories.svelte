<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import { implementInterviewFixture } from "#lib/run/fixtures/index.js";
  import { resetRemotes, setRemote } from "#lib/storybook/remotes.js";
  import { installStoryStream } from "#lib/storybook/stream.js";
  import Page from "./+page.svelte";

  installStoryStream();

  const data = () => ({
    snapshot: structuredClone(implementInterviewFixture.snapshot),
    graph: JSON.stringify(implementInterviewFixture.graph),
  });

  const { Story } = defineMeta({
    title: "Gimble/Run/Run page",
    component: Page,
    parameters: { layout: "fullscreen" },
    beforeEach: () => resetRemotes(),
  });
</script>

<!-- The whole route, with its remote functions answered by
     src/lib/storybook/mocks. Steer lands, stop and cancel are accepted. -->
<!-- Rendered as a child, not through args: Storybook wraps args in a proxy
     the page cannot structuredClone. -->
<Story name="Live run" asChild><Page data={data()} /></Story>

<Story
  name="Steer is dropped"
  asChild
  beforeEach={() => {
    setRemote("steer", async () => ({ landed: false }));
  }}
>
  <Page data={data()} />
</Story>

<Story
  name="Stop turn fails"
  asChild
  beforeEach={() => {
    setRemote("stopTurn", async () => {
      throw new Error("the run's owner is not reachable");
    });
  }}
>
  <Page data={data()} />
</Story>
