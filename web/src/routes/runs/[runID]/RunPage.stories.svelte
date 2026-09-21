<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import { implementInterviewFixture, issue325Fixture } from "#lib/run/fixtures/index.js";
  import { resetRemotes, setRemote } from "#lib/storybook/remotes.js";
  import { installStoryStream } from "#lib/storybook/stream.js";
  import Page from "./+page.svelte";

  installStoryStream();

  // The issue 325 fixture has real transcripts, a long assignment and a failed
  // command, so it is what the page is worth looking at with.
  const data = () => ({
    snapshot: structuredClone(issue325Fixture.snapshot),
    graph: JSON.stringify(issue325Fixture.graph),
  });

  const interviewData = () => ({
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

<Story name="Live run · interview workflow" asChild><Page data={interviewData()} /></Story>

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
