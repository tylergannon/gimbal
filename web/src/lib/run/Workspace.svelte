<script lang="ts">
  import type { Snippet } from "svelte";
  import Splitter from "./Splitter.svelte";
  import {
    MIN_MAP_WIDTH,
    MIN_WIDTH,
    clampWidth,
    readStoredMaximized,
    readStoredWidth,
    storeMaximized,
    storeWidth,
  } from "./paneSize.js";

  const NARROW_BREAKPOINT = 1024;

  let {
    map,
    pane,
    open = false,
    onclose,
    initialWidth,
    initialMaximized,
  }: {
    /** The map, rendered as-is; Workspace only hides it with CSS while maximized. */
    map: Snippet;
    /** The detail pane. Receives the current width, maximized flag, and the maximize toggle. */
    pane: Snippet<[{ width: number; maximized: boolean; onmaximize: () => void }]>;
    /** Whether something is selected — drives the narrow-screen overlay's visibility. */
    open?: boolean;
    /** Called to clear the selection: the narrow overlay's Escape and scrim-click. */
    onclose?: () => void;
    /** Overrides the remembered width for the first render only (stories, tests). */
    initialWidth?: number;
    /** Overrides the remembered maximized flag for the first render only (stories, tests). */
    initialMaximized?: boolean;
  } = $props();

  let width = $state(480);
  let maximized = $state(false);
  let viewportWidth = $state(1280);

  $effect(() => {
    width = clampWidth(initialWidth ?? readStoredWidth(), window.innerWidth);
    maximized = initialMaximized ?? readStoredMaximized();
    viewportWidth = window.innerWidth;
    const onresize = () => {
      viewportWidth = window.innerWidth;
      width = clampWidth(width, window.innerWidth);
    };
    window.addEventListener("resize", onresize);
    return () => window.removeEventListener("resize", onresize);
  });

  const narrow = $derived(viewportWidth < NARROW_BREAKPOINT);
  const maxWidth = $derived(Math.max(MIN_WIDTH, viewportWidth - MIN_MAP_WIDTH));

  function toggleMaximize() {
    maximized = !maximized;
    storeMaximized(maximized);
  }

  function resize(next: number) {
    width = clampWidth(next, viewportWidth);
  }

  function commit(next: number) {
    width = clampWidth(next, viewportWidth);
    storeWidth(width);
  }

  function isEditable(target: EventTarget | null) {
    const element = target as HTMLElement | null;
    return Boolean(element?.closest("input, textarea, [contenteditable=true]"));
  }

  function dialogOpen() {
    return Boolean(document.querySelector('[role="dialog"], [role="alertdialog"]'));
  }

  function handleKeydown(event: KeyboardEvent) {
    if (isEditable(event.target) || dialogOpen()) return;
    if (event.key === "\\") {
      event.preventDefault();
      toggleMaximize();
    } else if (event.key === "Escape") {
      if (maximized) {
        event.preventDefault();
        maximized = false;
        storeMaximized(false);
      } else if (narrow && open) {
        event.preventDefault();
        onclose?.();
      }
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="workspace-body">
  <div class="map-area" class:hidden={maximized}>
    {@render map()}
  </div>

  {#if narrow}
    {#if open}
      <button type="button" class="scrim" aria-label="Close the detail pane" onclick={onclose}></button>
      <div class="overlay">
        {@render pane({ width, maximized, onmaximize: toggleMaximize })}
      </div>
    {/if}
  {:else}
    {#if !maximized}
      <Splitter {width} min={MIN_WIDTH} max={maxWidth} onresize={resize} oncommit={commit} />
    {/if}
    <div class="pane-area" class:maximized>
      {@render pane({ width, maximized, onmaximize: toggleMaximize })}
    </div>
  {/if}
</div>

<style>
  .workspace-body {
    position: relative;
    display: flex;
    min-width: 0;
    min-height: 0;
    flex: 1;
  }

  .map-area {
    display: flex;
    min-width: 0;
    flex: 1;
  }

  .map-area.hidden {
    display: none;
  }

  .pane-area {
    display: flex;
    min-width: 0;
  }

  .pane-area.maximized {
    flex: 1;
  }

  .scrim {
    position: fixed;
    inset: 56px 0 0 0;
    z-index: 20;
    padding: 0;
    cursor: default;
    background: color-mix(in oklch, var(--foreground) 40%, transparent);
    border: 0;
  }

  .overlay {
    position: fixed;
    top: 56px;
    right: 0;
    bottom: 0;
    z-index: 21;
    display: flex;
    width: min(92vw, 560px);
    box-shadow: var(--shadow-lg);
  }

  .overlay :global(.detail) {
    width: 100% !important;
    min-width: 0 !important;
    border-left: none !important;
  }
</style>
