<script module lang="ts">
  import { defineMeta } from "@storybook/addon-svelte-csf";
  import Pip from "./Pip.svelte";

  const { Story } = defineMeta({
    title: "Gimbal/Run/Pip",
    component: Pip,
    parameters: {
      layout: "centered",
    },
    argTypes: {
      state: {
        control: "select",
        options: ["ended", "running", "failed", "waiting", "not-yet"],
      },
    },
  });
</script>

<Story name="All states" asChild>
  <div class="states" aria-label="Pip states">
    <span><Pip state="ended" />ended</span>
    <span><Pip state="running" />running</span>
    <span><Pip state="failed" />failed</span>
    <span><Pip state="waiting" />waiting for you</span>
    <span><Pip state="not-yet" />not yet</span>
  </div>
</Story>

<Story name="Ended" args={{ state: "ended" }} />
<Story name="Running" args={{ state: "running" }} />
<Story name="Failed" args={{ state: "failed" }} />
<Story name="Waiting for you" args={{ state: "waiting" }} />
<Story name="Not yet" args={{ state: "not-yet" }} />

<style>
  .states {
    display: flex;
    align-items: center;
    gap: 14px;
    color: var(--foreground);
    font-size: 13px;
    line-height: 18px;
  }

  .states span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }
</style>
