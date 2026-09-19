<script module lang="ts">
  export type GroupBranchPlacement = {
    name: string;
    centerX: number;
    top: number;
    bottom: number;
  };
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import type { Group as WorkflowGroup } from "../workflow/types.js";

  let {
    group,
    width,
    height,
    forkY,
    joinY,
    branches,
    children,
  }: {
    group: WorkflowGroup & { kind: "group" };
    width: number;
    height: number;
    forkY: number;
    joinY: number;
    branches: GroupBranchPlacement[];
    children?: Snippet;
  } = $props();

  const firstCenter = $derived(branches.at(0)?.centerX ?? width / 2);
  const lastCenter = $derived(branches.at(-1)?.centerX ?? width / 2);
</script>

<div class="group" style:width={`${width}px`} style:height={`${height}px`} data-group={group.name}>
  <svg {width} {height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
    <path class="line" d={`M${width / 2},0 V${forkY}`} />
    <path class="line" d={`M${firstCenter},${forkY} H${lastCenter}`} />
    {#each branches as branch}
      <path class="line" d={`M${branch.centerX},${forkY} V${branch.top}`} />
      <path class="line" d={`M${branch.centerX},${branch.bottom} V${joinY}`} />
    {/each}
    <path class="line" d={`M${firstCenter},${joinY} H${lastCenter}`} />
    <path class="line" d={`M${width / 2},${joinY} V${height}`} />
  </svg>
  {@render children?.()}
</div>

<style>
  .group {
    position: relative;
    pointer-events: none;
  }

  svg {
    position: absolute;
    inset: 0;
    overflow: visible;
  }

  .line {
    fill: none;
    stroke: var(--status-muted);
    stroke-width: 2;
  }
</style>
