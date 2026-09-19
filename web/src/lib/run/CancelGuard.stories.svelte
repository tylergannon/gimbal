<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import { Button } from "#lib/components/ui/button/index.js";
  import CancelGuard from "./CancelGuard.svelte";
  import { implementInterviewFixture } from "./fixtures/index.js";

  const run = implementInterviewFixture.snapshot.run;
  const turn = implementInterviewFixture.snapshot.turns["coding.1/turn.3"];
  if (!turn) throw new Error("coding turn is missing");

  const { Story } = defineMeta({
    title: "Gimble/Run/CancelGuard",
    component: CancelGuard,
    parameters: { layout: "fullscreen" },
  });
</script>

<script lang="ts">
  let open = $state(true);
  let event = $state("Choose how to interrupt the live run");
</script>

<Story name="Cancel run guard" asChild>
  <div class="story-frame">
    <div class="demo-card">
      <strong>implement-interview is running</strong>
      <span aria-live="polite">{event}</span>
      <Button variant="outline" onclick={() => (open = true)}>Cancel run…</Button>
    </div>
    <CancelGuard
      bind:open
      {run}
      {turn}
      onstopturn={(selectedTurn) => (event = `stop-turn · ${selectedTurn.id}`)}
      oncancelrun={(selectedRun) => (event = `cancel-run · ${selectedRun.id}`)}
    />
  </div>
</Story>

<style>
  .story-frame {
    display: grid;
    min-height: 100vh;
    place-items: center;
    color: var(--foreground);
    background-color: color-mix(in oklch, var(--background) 96%, var(--foreground));
    background-image: radial-gradient(
      color-mix(in oklch, var(--foreground) 20%, transparent) 1px,
      transparent 1px
    );
    background-size: 20px 20px;
  }

  .demo-card {
    display: flex;
    width: 340px;
    flex-direction: column;
    gap: 12px;
    padding: 18px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
  }

  .demo-card span {
    min-height: 36px;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
  }
</style>
