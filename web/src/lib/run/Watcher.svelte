<script lang="ts">
  import EyeIcon from "@lucide/svelte/icons/eye";
  import type { Supervisor } from "../workflow/types.js";
  import Pip, { type PipState } from "./Pip.svelte";

  let {
    supervisor,
    state,
  }: {
    supervisor: Supervisor;
    state: PipState;
  } = $props();
</script>

<div class="watcher" data-watcher={supervisor.session}>
  <EyeIcon size={14} />
  <span>{supervisor.session}</span>
  <Pip {state} />
</div>

<style>
  .watcher {
    display: flex;
    width: 100%;
    height: 100%;
    box-sizing: border-box;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    color: var(--foreground);
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 18px;
    white-space: nowrap;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line);
    border-radius: 8px;
    box-shadow: var(--shadow-xs);
  }

  .watcher > :global(svg) {
    flex-shrink: 0;
    color: var(--status-muted);
  }

  .watcher > span:first-of-type {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .watcher > :global(.pip) {
    margin-left: auto;
  }
</style>
