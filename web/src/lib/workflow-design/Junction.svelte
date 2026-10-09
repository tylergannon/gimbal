<script lang="ts">
  import { Handle, Position, type NodeProps } from '@xyflow/svelte';
  import type { WorkflowNode } from './types';
  let { data }: NodeProps<WorkflowNode> = $props();
  const positions = {NORTH:Position.Top,SOUTH:Position.Bottom,EAST:Position.Right,WEST:Position.Left};
</script>
<div class="junction" style:width={`${data.width}px`} style:height={`${data.height}px`}>
{#each data.ports ?? [] as port}<Handle id={port.id} type={port.side === 'NORTH' ? 'target' : 'source'} position={positions[port.side]} style={`left:${port.x}px;top:${port.y}px;opacity:0;pointer-events:none`} tabindex={-1} aria-hidden="true" isConnectable={false}/>{/each}
</div>
<style>.junction{background:#0b0c0c;border-radius:50%;}</style>
