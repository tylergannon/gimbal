<script module lang="ts">
  import type { Service } from "../workflow/types.js";

  export type ServiceItem = {
    service: Service;
    selectionKey: string;
    selected: boolean;
  };
</script>

<script lang="ts">
  import ServerIcon from "@lucide/svelte/icons/server";

  let {
    items,
    root = false,
    onselect,
  }: {
    items: ServiceItem[];
    root?: boolean;
    onselect?: (service: Service) => void;
  } = $props();
</script>

<section class="services" aria-label={root ? "Workflow services" : "Services"}>
  <div class="heading">
    <ServerIcon size={13} />
    <span>{root ? "Workflow services" : "Services"}</span>
  </div>
  <div class="list">
    {#each items as item}
      <button
        type="button"
        class:selected={item.selected}
        data-selection-key={item.selectionKey}
        aria-pressed={item.selected}
        aria-label={`Select service ${item.service.name}`}
        title={`${item.service.file}:${item.service.line}`}
        onclick={() => onselect?.(item.service)}
      >
        <ServerIcon size={13} aria-hidden="true" />
        <span>{item.service.name}</span>
      </button>
    {/each}
  </div>
</section>

<style>
  .services {
    width: 100%;
    height: 100%;
    box-sizing: border-box;
    padding: 6px;
    color: var(--card-foreground);
    background: color-mix(in oklch, var(--map-paper-2) 86%, var(--muted));
    border: 1px dashed var(--map-line);
    border-radius: 8px;
  }

  .heading {
    display: flex;
    height: 16px;
    align-items: center;
    gap: 6px;
    color: var(--status-muted);
    font-size: 11px;
    font-weight: 700;
    line-height: 16px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
  }

  button {
    display: flex;
    width: 100%;
    height: 26px;
    box-sizing: border-box;
    align-items: center;
    gap: 8px;
    padding: 0 8px;
    overflow: hidden;
    color: var(--foreground);
    font: inherit;
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 18px;
    text-align: left;
    cursor: pointer;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line-soft);
    border-radius: 6px;
  }

  button:hover {
    border-color: var(--map-line-strong);
  }

  button:focus-visible {
    outline: 2px solid var(--status-live);
    outline-offset: 2px;
  }

  button.selected {
    border-color: var(--status-running);
    box-shadow: 0 0 0 2px var(--map-live-soft);
  }

  button span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
