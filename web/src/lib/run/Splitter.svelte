<script lang="ts">
  let {
    width,
    min,
    max,
    onresize,
    oncommit,
  }: {
    width: number;
    min: number;
    max: number;
    onresize?: (width: number) => void;
    oncommit?: (width: number) => void;
  } = $props();

  const STEP = 24;

  let dragging = $state(false);
  let dragOrigin: { x: number; width: number } | undefined;

  function clamp(next: number) {
    return Math.min(max, Math.max(min, next));
  }

  function startDrag(event: PointerEvent) {
    if (event.button !== 0) return;
    dragging = true;
    dragOrigin = { x: event.clientX, width };
    const target = event.currentTarget as HTMLElement;
    target.setPointerCapture(event.pointerId);
    // preventDefault (to stop text selection while dragging) also suppresses
    // the browser's default focus-on-pointerdown, so focus explicitly —
    // keyboard resize must still work right after a drag.
    target.focus();
    event.preventDefault();
  }

  function drag(event: PointerEvent) {
    if (!dragging || !dragOrigin) return;
    // The pane sits to the right of the handle, so dragging left grows it.
    const delta = dragOrigin.x - event.clientX;
    onresize?.(clamp(dragOrigin.width + delta));
  }

  function endDrag(event: PointerEvent) {
    if (!dragging) return;
    dragging = false;
    dragOrigin = undefined;
    const target = event.currentTarget as HTMLElement;
    if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId);
    oncommit?.(width);
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      const next = clamp(width + STEP);
      onresize?.(next);
      oncommit?.(next);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      const next = clamp(width - STEP);
      onresize?.(next);
      oncommit?.(next);
    } else if (event.key === "Home") {
      event.preventDefault();
      onresize?.(min);
      oncommit?.(min);
    } else if (event.key === "End") {
      event.preventDefault();
      onresize?.(max);
      oncommit?.(max);
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="splitter"
  class:dragging
  role="separator"
  aria-orientation="vertical"
  aria-label="Resize the detail pane"
  aria-valuenow={Math.round(width)}
  aria-valuemin={Math.round(min)}
  aria-valuemax={Math.round(max)}
  tabindex="0"
  onpointerdown={startDrag}
  onpointermove={drag}
  onpointerup={endDrag}
  onpointercancel={endDrag}
  onkeydown={keydown}
>
  <span class="line"></span>
</div>

<style>
  .splitter {
    position: relative;
    z-index: 5;
    width: 6px;
    min-height: 120px;
    flex-shrink: 0;
    cursor: col-resize;
    touch-action: none;
  }

  .line {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 2px;
    width: 2px;
    background: var(--map-line);
  }

  .splitter:hover .line,
  .splitter:focus-visible .line,
  .splitter.dragging .line {
    background: var(--status-running);
  }

  .splitter:focus-visible {
    outline: none;
  }
</style>
