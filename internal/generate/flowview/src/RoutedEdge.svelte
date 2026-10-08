<script lang="ts">
  import { BaseEdge, EdgeLabel, type EdgeProps } from '@xyflow/svelte';
  import type { WorkflowEdge } from './types';
  let { id, data }: EdgeProps<WorkflowEdge> = $props();
  let masks = $derived((data?.headerMasks ?? []) as {x:number;y:number;width:number;height:number}[]);
  let points = $derived((data?.sections ?? []).flatMap(s=>[s.startPoint,...s.bendPoints ?? [],s.endPoint]));
  let bounds = $derived({x:Math.min(...points.map(p=>p.x))-12,y:Math.min(...points.map(p=>p.y))-12,width:Math.max(...points.map(p=>p.x))-Math.min(...points.map(p=>p.x))+24,height:Math.max(...points.map(p=>p.y))-Math.min(...points.map(p=>p.y))+24});
  let paths = $derived((data?.sections ?? []).map(section => {
    const points = [section.startPoint, ...(section.bendPoints ?? []), section.endPoint];
    return points.map((p, i) => `${i ? 'L' : 'M'} ${p.x} ${p.y}`).join(' ');
  }));
</script>
<defs><mask id="mask-{id}" maskUnits="userSpaceOnUse" x={bounds.x} y={bounds.y} width={bounds.width} height={bounds.height}><rect x={bounds.x} y={bounds.y} width={bounds.width} height={bounds.height} fill="white"/>{#each masks as rect}<rect x={rect.x} y={rect.y} width={rect.width} height={rect.height} fill="black"/>{/each}</mask><marker id="arrow-{id}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M 1 1 L 9 5 L 1 9" fill="none" stroke="#0b0c0c" stroke-width="1.5"/></marker></defs>
<g mask={`url(#mask-${id})`}>
{#each paths as path, i}
  <BaseEdge id={`${id}-${i}`} {path} style="stroke:#0b0c0c;stroke-width:1.7" markerEnd={i === paths.length - 1 ? `url(#arrow-${id})` : undefined} interactionWidth={0}/>
{/each}
</g>
{#each data?.labels ?? [] as label}
  <EdgeLabel x={label.x + label.width / 2} y={label.y + label.height / 2}>
    <span class="edge-caption">{label.text}</span>
  </EdgeLabel>
{/each}
<style>
  .edge-caption { display:block; background:#fff; color:#0b0c0c; padding:3px 6px; font-size:13px; line-height:18px; white-space:nowrap; }
</style>
