<script lang="ts">
  import { Handle, Position, type NodeProps } from '@xyflow/svelte';
  import Pip from '../run/Pip.svelte';
  import type { RunMark, WatcherMark } from '../run/runSource';
  import type { WorkflowNode } from './types';
  let { id, data }: NodeProps<WorkflowNode> = $props();
  const positions = {NORTH: Position.Top, SOUTH: Position.Bottom, EAST: Position.Right, WEST: Position.Left};
  let originalKind = $derived(data.sourceNode?.kind ?? data.kind);
  let decision = $derived(data.kind === 'Condition' || data.kind === 'decision');
  let container = $derived(data.kind === 'container');
  let writes = $derived(!!data.members);
  let context = $derived(['Set','SetJSON','writes'].includes(originalKind) || !!data.members);
  let control = $derived(['Repeat','Iterate','PromiseLoop','Group','Go','Scope','Condition'].includes(originalKind));
  let color = $derived(context ? 'var(--hue-context)' : control ? 'var(--hue-control)' : 'var(--hue-session)');
  // Run state, present only when the run map draws this node.
  let run = $derived(data.run as RunMark | undefined);
  let runMode = $derived(!!data.onpick);
  let now = $derived(run?.state === 'running' || run?.state === 'waiting');
  let muted = $derived(run?.state === 'not-yet');
  function activate() { (data.onactivate as ((id:string)=>void) | undefined)?.(id); }
  function press() { if (runMode) (data.onpick as (id:string)=>void)(id); else activate(); }
  function open() { (data.onopen as ((id:string)=>void) | undefined)?.(id); }
  function peek(event: MouseEvent | FocusEvent, focus=false) { (data.onpeek as ((id:string,rect:DOMRect,focus:boolean)=>void) | undefined)?.(id, (event.currentTarget as HTMLElement).getBoundingClientRect(), focus); }
  function unpeek() { (data.onunpeek as (()=>void) | undefined)?.(); }
  function instance(event: Event) { (data.oninstance as (group:string,key:string)=>void)?.(run!.instanceGroup!, (event.currentTarget as HTMLSelectElement).value); }
  function watch(watcher: WatcherMark) { (data.onwatcher as (w:WatcherMark)=>void)?.(watcher); }
</script>
<div class="node" class:container class:decision class:writes class:terminal={data.kind==='Return'} class:chosen={!!data.chosen} class:now class:muted class:failed={run?.state==='failed'} style:width={`${data.width}px`} style:height={`${data.height}px`} style:--accent={color} data-selection-key={run?.selectionKey}>
  {#if decision}
    <svg class="silhouette" viewBox={`0 0 ${data.width} ${data.height}`} aria-hidden="true"><polygon points={`${data.width/2},1 ${data.width-1},${data.height/2} ${data.width/2},${data.height-1} 1,${data.height/2}`}/></svg>
  {/if}
  <button class="node-button nodrag nopan" class:header={container || writes && data.expanded} onclick={press} ondblclick={open} onmouseenter={peek} onmouseleave={unpeek} onfocus={(event)=>peek(event,true)} onblur={unpeek} aria-expanded={data.expandable && !runMode ? data.expanded : undefined} aria-pressed={runMode ? !!data.chosen : undefined} aria-label={data.expandable && !runMode ? `${data.label}, ${data.expanded ? 'collapse' : 'expand'}` : run ? `${data.label}, ${run.state}${run.meta ? `, ${run.meta}` : ''}` : data.label}>
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
    {#if run && !decision}<Pip state={run.state} />{/if}
    <span class="label">{data.label}</span>
    {#if run?.meta && !(container && (run.instances?.length ?? 0) > 1)}<span class="meta">{run.meta}</span>{/if}
    {#if data.expandable && !runMode}<span class="chevron" aria-hidden="true">{data.expanded ? '⌄' : '›'}</span>{/if}
  </button>
  {#if runMode && data.expandable}
    <button class="toggle nodrag nopan" class:open={data.expanded} onclick={activate} aria-expanded={data.expanded} aria-label={`${data.expanded ? 'Collapse' : 'Expand'} ${data.label}`}>{data.expanded ? '⌄' : '›'}</button>
  {/if}
  {#if container && run?.instances && run.instances.length > 1}
    <select class="instances nodrag nopan" aria-label={`${data.label} instance`} value={run.instance} onchange={instance}>
      {#each run.instances as scope, index (scope.key)}<option value={scope.key}>{index + 1} of {run.instances.length}{scope.status === 'running' ? ' · running' : scope.error ? ' · failed' : ''}</option>{/each}
    </select>
  {/if}
  {#if run?.watchers.length}
    <div class="watchers" aria-label={`Watching ${data.label}`}>
      {#each run.watchers as watcher (watcher.selectionKey)}
        <button class="watcher nodrag nopan" class:chosen={data.chosenKey === watcher.selectionKey} onclick={() => watch(watcher)} aria-label={`${watcher.supervisor.session} watcher, ${watcher.state}`}>
          <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M1.5 10S5 4 10 4s8.5 6 8.5 6-3.5 6-8.5 6-8.5-6-8.5-6Z"/><circle cx="10" cy="10" r="2.5"/></svg>
          <Pip state={watcher.state} /><span class="label">{watcher.supervisor.session}</span>
        </button>
      {/each}
    </div>
  {/if}
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
.node{position:relative;color:var(--graph-ink);background:var(--graph-fill);border:1.5px solid var(--graph-stroke);border-radius:4px;box-sizing:border-box;}
.node.chosen{outline:3px solid var(--chosen);outline-offset:2px;background:var(--chosen-soft)}
.node.container{border-color:var(--graph-stroke-container);border-radius:8px;background:transparent;}
.node.decision{border:0;background:transparent;outline:0}
.node.now:not(.container):not(.decision){border-color:var(--status-running);box-shadow:0 0 0 1px var(--status-running)}
.node.now:not(.container):not(.decision)::before{content:"";position:absolute;left:-14px;top:50%;margin-top:-5px;border:5px solid transparent;border-left:7px solid var(--status-running)}
.node.failed:not(.container){border-color:var(--status-failed)}
.node.muted{opacity:.55}
.silhouette{position:absolute;inset:0;width:100%;height:100%;overflow:visible;pointer-events:none}
.silhouette polygon{fill:var(--decision-fill);stroke:var(--decision-stroke);stroke-width:1.7}
.chosen .silhouette polygon{stroke:var(--chosen);stroke-width:3}
.node-button{position:relative;border:0;background:transparent;display:flex;align-items:center;gap:9px;width:100%;height:100%;padding:8px 12px;text-align:left;color:inherit;cursor:pointer;font:inherit;font-size:15px;line-height:20px;font-weight:550;}
.node-button:focus-visible{outline:3px solid var(--graph-ink);box-shadow:0 0 0 6px var(--focus-ring);border-radius:3px}
.node-button:hover{background:var(--chosen-soft)}
.node.terminal{border-radius:18px}.node-button.header{height:40px;background:var(--graph-header);border-radius:7px 7px 0 0;border-bottom:1px solid var(--graph-header-rule)}
.icon{width:19px;height:19px;flex:none;color:var(--accent)}
.label{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
.meta{flex:none;font-size:12px;font-weight:400;color:var(--ink-2);white-space:nowrap}
.chevron{font-size:23px;font-weight:400;line-height:16px;margin-left:4px}
.toggle{position:absolute;top:0;right:0;width:36px;height:40px;border:0;background:transparent;color:inherit;font-size:23px;line-height:16px;cursor:pointer}
.toggle:hover{background:var(--chosen-soft)}
.container .node-button.header,.node:has(.toggle) .node-button{padding-right:40px}
.instances{position:absolute;top:8px;right:40px;height:24px;max-width:140px;font-size:12px;color:var(--graph-ink);background:var(--graph-fill);border:1px solid var(--graph-stroke-container);border-radius:4px}
.decision .node-button{justify-content:center;text-align:center;padding:8px 40px;font-size:14px;line-height:18px;}
.decision .node-button:hover{background:transparent}
.decision .label{white-space:normal;overflow:visible;flex:0 1 auto}
.decision .chevron{position:absolute;top:0;right:0;left:auto;bottom:auto;font-size:18px}
.decision .toggle{top:0;right:-8px;height:32px;font-size:18px}
.watchers{position:absolute;right:calc(100% + 30px);top:2px;display:flex;flex-direction:column;gap:6px}
.watchers::after{content:"";position:absolute;left:100%;top:16px;width:28px;border-top:1.5px dotted var(--graph-stroke)}
.watcher{display:flex;align-items:center;gap:7px;height:32px;max-width:220px;padding:0 10px;font-size:13px;font-weight:500;color:var(--graph-ink);background:var(--graph-fill);border:1.5px dashed var(--graph-stroke);border-radius:16px;cursor:pointer;white-space:nowrap}
.watcher .icon{width:16px;height:16px;color:var(--hue-session)}
.watcher.chosen{outline:3px solid var(--chosen);outline-offset:2px}
.write-rows{padding:2px 0}.write-row{display:block;border:0;border-top:1px solid var(--row-rule);background:transparent;width:100%;text-align:left;padding:5px 14px;height:32px;font:13px/22px ui-monospace,SFMono-Regular,monospace;color:inherit;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;cursor:pointer}.write-row:hover{background:var(--chosen-soft)}.write-row.selected{background:var(--row-selected);box-shadow:inset 3px 0 var(--chosen)}.writes .header{background:var(--graph-header-writes)}
</style>
