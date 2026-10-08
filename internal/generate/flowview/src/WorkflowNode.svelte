<script lang="ts">
  import { Handle, Position, type NodeProps } from '@xyflow/svelte';
  import type { WorkflowNode } from './types';
  let { id, data }: NodeProps<WorkflowNode> = $props();
  const positions = {NORTH: Position.Top, SOUTH: Position.Bottom, EAST: Position.Right, WEST: Position.Left};
  let originalKind = $derived(data.sourceNode?.kind ?? data.kind);
  let decision = $derived(data.kind === 'Condition' || data.kind === 'decision');
  let container = $derived(data.kind === 'container');
  let writes = $derived(!!data.members);
  let context = $derived(['Set','SetJSON','writes'].includes(originalKind) || !!data.members);
  let control = $derived(['Repeat','Iterate','PromiseLoop','Group','Go','Scope','Condition'].includes(originalKind));
  let color = $derived(context ? '#106165' : control ? '#54319f' : '#1a65a6');
  function activate() { (data.onactivate as ((id:string)=>void) | undefined)?.(id); }
  function peek(event: MouseEvent | FocusEvent) { (data.onpeek as ((id:string,rect:DOMRect)=>void) | undefined)?.(id, (event.currentTarget as HTMLElement).getBoundingClientRect()); }
  function unpeek() { (data.onunpeek as (()=>void) | undefined)?.(); }
</script>
<div class="node" class:container class:decision class:writes class:terminal={data.kind==='Return'} class:chosen={!!data.chosen} style:width={`${data.width}px`} style:height={`${data.height}px`} style:--accent={color}>
  {#if decision}
    <svg class="silhouette" viewBox={`0 0 ${data.width} ${data.height}`} aria-hidden="true"><polygon points={`${data.width/2},1 ${data.width-1},${data.height/2} ${data.width/2},${data.height-1} 1,${data.height/2}`}/></svg>
  {/if}
  <button class="node-button nodrag nopan" class:header={container || writes && data.expanded} onclick={activate} onmouseenter={peek} onmouseleave={unpeek} onfocus={peek} onblur={unpeek} aria-expanded={data.expandable ? data.expanded : undefined} aria-label={data.expandable ? `${data.label}, ${data.expanded ? 'collapse' : 'expand'}` : data.label}>
    {#if !decision}
      <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true">
        {#if data.kind==='Return'}<path d="M16 3v14M3 10h10M9 6l4 4-4 4"/>
        {:else if context}<path d="M3 4h14v4H3zM3 12h14v4H3z"/>
        {:else if ['Repeat','Iterate','PromiseLoop'].includes(originalKind)}<path d="M16 7a6.5 6.5 0 1 0 .4 5M16 2v5h-5"/>
        {:else if originalKind === 'Group'}<path d="M10 2v4M3 11V6h14v5M10 6v5M3 14v4M10 14v4M17 14v4"/>
        {:else if ['Go','Scope'].includes(originalKind)}<path d="M6 2H2v16h4M14 2h4v16h-4M7 10h6"/>
        {:else if originalKind === 'NewSession' || originalKind === 'Fork'}<path d="M4 4h12v9H8l-4 4V4Z"/>
        {:else if originalKind === 'Command'}<path d="m3 5 5 5-5 5m7 0h7"/>
        {:else if originalKind === 'Service'}<rect x="3" y="3" width="14" height="14" rx="1"/><path d="M3 10h14M6 6h1m-1 7h1"/>
        {:else}<path d="M10 2 12 7l5 3-5 2-2 6-2-6-5-2 5-3Z"/>{/if}
      </svg>
    {/if}
    <span class="label">{data.label}</span>
    {#if data.expandable}<span class="chevron" aria-hidden="true">{data.expanded ? '⌄' : '›'}</span>{/if}
  </button>
  {#if writes && data.expanded}
    <div class="write-rows">
      {#each data.members ?? [] as member}
        <button class="write-row nodrag nopan" class:selected={data.selectedId===member.id} onclick={()=>(data.onselect as ((node:typeof member)=>void))?.(member)}>{member.title || member.label}</button>
      {/each}
    </div>
  {/if}
  {#each data.ports ?? [] as port}
    <Handle id={port.id} type={port.side === 'NORTH' ? 'target' : 'source'} position={positions[port.side]} style={`left:${port.x}px;top:${port.y}px;opacity:0;pointer-events:none`} tabindex={-1} aria-hidden="true" isConnectable={false}/>
  {/each}
</div>
<style>
.node{position:relative;color:#0b0c0c;background:white;border:1.5px solid #484949;border-radius:4px;box-sizing:border-box;}
.node.chosen{outline:3px solid #1a65a6;outline-offset:2px;background:#f3f8fc}
.node.container{border-color:#717272;border-radius:8px;background:transparent;}
.node.decision{border:0;background:transparent;outline:0}
.silhouette{position:absolute;inset:0;width:100%;height:100%;overflow:visible;pointer-events:none}
.silhouette polygon{fill:#fffaf0;stroke:#76520e;stroke-width:1.7}
.chosen .silhouette polygon{stroke:#1a65a6;stroke-width:3}
.node-button{position:relative;border:0;background:transparent;display:flex;align-items:center;gap:9px;width:100%;height:100%;padding:8px 12px;text-align:left;color:inherit;cursor:pointer;font:inherit;font-size:15px;line-height:20px;font-weight:550;}
.node-button:focus-visible{outline:3px solid #0b0c0c;box-shadow:0 0 0 6px #ffdd00;border-radius:3px}
.node-button:hover{background:#f3f8fc}
.node.terminal{border-radius:18px}.node-button.header{height:40px;background:#f6f5fa;border-radius:7px 7px 0 0;border-bottom:1px solid #aaa}
.icon{width:19px;height:19px;flex:none;color:var(--accent)}
.label{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
.chevron{font-size:23px;font-weight:400;line-height:16px;margin-left:4px}
.decision .node-button{justify-content:center;text-align:center;padding:8px 40px;font-size:14px;line-height:18px;}
.decision .node-button:hover{background:transparent}
.decision .label{white-space:normal;overflow:visible;flex:0 1 auto}
.decision .chevron{position:absolute;top:0;right:0;left:auto;bottom:auto;font-size:18px}
.write-rows{padding:2px 0}.write-row{display:block;border:0;border-top:1px solid #e1e3e3;background:transparent;width:100%;text-align:left;padding:5px 14px;height:32px;font:13px/22px ui-monospace,SFMono-Regular,monospace;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;cursor:pointer}.write-row:hover{background:#f3f8fc}.write-row.selected{background:#e8f1f8;box-shadow:inset 3px 0 #1a65a6}.writes .header{background:#f3f8f7}
</style>
