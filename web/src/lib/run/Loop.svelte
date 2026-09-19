<script lang="ts">
  import type { Snippet } from "svelte";
  import type { PromiseLoop } from "../workflow/types.js";

  let {
    loop,
    width,
    height,
    centerX,
    plannerRight,
    plannerCenterY,
    bodyRight,
    bodyExitY,
    returnX,
    returnBottomY,
    children,
  }: {
    loop: PromiseLoop & { kind: "promise_loop" };
    width: number;
    height: number;
    centerX: number;
    plannerRight: number;
    plannerCenterY: number;
    bodyRight: number;
    bodyExitY: number;
    returnX: number;
    returnBottomY: number;
    children?: Snippet;
  } = $props();

  const marker = $props.id();
  const path = $derived(
    `M${centerX},${bodyExitY} V${returnBottomY} H${returnX} V${plannerCenterY} H${plannerRight}`,
  );
</script>

<div
  class="loop"
  style:width={`${width}px`}
  style:height={`${height}px`}
  data-loop={loop.name}
  data-return-margin={returnX - bodyRight}
>
  <svg {width} {height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
    <defs>
      <marker
        id={marker}
        viewBox="0 0 10 10"
        refX="9"
        refY="5"
        markerWidth="7"
        markerHeight="7"
        orient="auto"
      >
        <path d="M0,0 L10,5 L0,10 z" />
      </marker>
    </defs>
    <path class="return" d={path} marker-end={`url(#${marker})`} />
  </svg>
  {@render children?.()}
</div>

<style>
  .loop {
    position: relative;
    pointer-events: none;
  }

  svg {
    position: absolute;
    inset: 0;
    overflow: visible;
  }

  .return {
    fill: none;
    stroke: var(--status-muted);
    stroke-width: 1.5;
  }

  marker path {
    fill: var(--status-muted);
  }
</style>
