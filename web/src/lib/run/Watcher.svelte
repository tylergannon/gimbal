<script lang="ts">
  import EyeIcon from "@lucide/svelte/icons/eye";
  import type { Supervisor } from "../workflow/types.js";
  import Pip, { type PipState } from "./Pip.svelte";

  let {
    supervisor,
    state,
    selected = false,
    onselect,
  }: {
    supervisor: Supervisor;
    state: PipState;
    selected?: boolean;
    onselect?: (supervisor: Supervisor) => void;
  } = $props();
</script>

<button
  type="button"
  class="watcher"
  class:selected
  data-watcher={supervisor.session}
  aria-pressed={selected}
  aria-label={`Select watcher ${supervisor.session}`}
  onclick={() => onselect?.(supervisor)}
>
  <EyeIcon size={14} />
  <span>{supervisor.session}</span>
  <Pip {state} />
</button>

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
    font: inherit;
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.02em;
    line-height: 18px;
    white-space: nowrap;
    cursor: pointer;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line);
    border-radius: 8px;
    box-shadow: var(--shadow-xs);
  }

  .watcher:hover {
    border-color: var(--map-line-strong);
  }

  .watcher:focus-visible {
    outline: 2px solid var(--status-live);
    outline-offset: 2px;
  }

  .watcher.selected {
    border-color: var(--status-running);
    box-shadow: 0 0 0 3px var(--map-live-soft), var(--shadow-xs);
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
