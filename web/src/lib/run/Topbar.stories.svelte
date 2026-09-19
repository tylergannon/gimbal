<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import type { RunStatus } from "../observation/index.js";
  import { implementInterviewFixture } from "./fixtures/index.js";
  import Topbar from "./Topbar.svelte";

  const run = implementInterviewFixture.snapshot.run;
  const statuses: RunStatus[] = ["running", "completed", "failed", "cancelled"];

  const { Story } = defineMeta({
    title: "Gimble/Run/Topbar",
    component: Topbar,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let event = $state("Use a topbar control");
</script>

<Story name="Live controls" asChild>
  <div class="story-frame">
    <Topbar
      {run}
      elapsed="48m 12s"
      onsearch={(query) => (event = `search · ${query || "empty"}`)}
      oncurrentselect={() => (event = "selected · coding · task 3")}
      onstopturn={() => (event = "stop-turn · coding.1 / turn.3")}
      oncancelrun={() => (event = `cancel-run · ${run.id}`)}
    />
    <p aria-live="polite">{event}</p>
  </div>
</Story>

<Story name="All statuses" asChild>
  <div class="status-frame">
    {#each statuses as status}
      <Topbar
        run={{ ...run, status }}
        connection={status === "failed" ? "disconnected" : "live"}
        elapsed={status === "running" ? "48m 12s" : "52m 03s"}
        stopDisabled={status !== "running"}
      />
    {/each}
    <Topbar {run} connection="disconnected" elapsed="48m 12s" />
  </div>
</Story>

<style>
  .story-frame,
  .status-frame {
    min-height: 100vh;
    color: var(--foreground);
    background: var(--background);
  }

  p {
    padding: 12px 16px;
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .status-frame {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding-bottom: 16px;
  }
</style>
