<script lang="ts">
  import Shape from './Shape.svelte';
  import type { ViewNode } from './types';
  let {node,onclose,goTypes,ontypes,openPaths,ondetail}: {
    node:ViewNode;onclose:()=>void;goTypes:boolean;ontypes:(value:boolean)=>void;
    openPaths:string[];ondetail:(path:string,open:boolean)=>void;
  }=$props();
  const escapePath=(name:string)=>name.replaceAll('~','~0').replaceAll('/','~1');
  function toggle(event:MouseEvent,path:string){event.preventDefault();ondetail(path,!openPaths.includes(path));}
  const kinds:Record<string,string>={Generate:'Prompt',NewSession:'Session',Set:'Context',SetJSON:'Context',Condition:'Decision',Repeat:'Loop',Group:'Parallel work',Go:'Branch',Command:'Command',Service:'Service',Scope:'Scope'};
</script>
<aside aria-label="Step details">
  <header><span>{kinds[node.kind] ?? node.kind}</span><button onclick={onclose} aria-label="Close details">×</button></header>
  <h2>{node.title || node.label || kinds[node.kind]}</h2>
  {#if node.description}<p class="description">{node.description}</p>{/if}
  {#if node.kind==='Generate'}
    <section><h3>{node.detail?.promptKnown ? 'Prompt' : 'Prompt expression'}</h3><pre>{node.detail?.promptKnown ? node.prompt : node.detail?.promptExpression || node.prompt}</pre></section>
  {/if}
  {#if node.kind==='Set' || node.kind==='SetJSON'}
    <section><h3>{node.detail?.keyKnown ? node.label : node.detail?.keyExpression}</h3>
    {#if node.detail?.shape}<Shape name="value" shape={node.detail.shape} {goTypes} path="/shape" {openPaths} {ondetail}/>{/if}
    <details class="source" open={openPaths.includes('/value')}><summary onclick={(e)=>toggle(e,'/value')}>Value expression</summary><pre>{node.detail?.valueExpression}</pre></details></section>
  {/if}
  {#if node.context?.length && node.kind!=='Set' && node.kind!=='SetJSON'}
    <section><div class="section-head"><h3>Context shape</h3><label><input type="checkbox" checked={goTypes} onchange={(e)=>ontypes(e.currentTarget.checked)}/> Go types</label></div>
      {#each node.context as field (field.key)}{#if field.shape}<Shape name={field.key} shape={field.shape} optional={field.optional} {goTypes} path={'/context/'+escapePath(field.key)} {openPaths} {ondetail}/>{/if}{/each}
    </section>
  {:else if node.kind==='Generate'}<p class="empty">No context set before this step.</p>{/if}
  {#if node.note}<p class="description">{node.note}</p>{/if}
  <details class="source" open={openPaths.includes('/source')}><summary onclick={(e)=>toggle(e,'/source')}>Go source <span>{node.source?.file?.split('/').at(-1)}:{node.source?.line}</span></summary><pre>{node.control?.code || node.detail?.expression || node.label}</pre></details>
</aside>
<style>
aside{height:100%;overflow:auto;box-sizing:border-box;padding:18px 22px 28px;background:#fff;border-left:1px solid #b1b4b6}
header{display:flex;align-items:center;justify-content:space-between;color:#484949;font-size:13px}header button{font-size:24px;border:0;background:transparent;padding:0 8px;cursor:pointer;color:#0b0c0c}
h2{font-size:21px;line-height:1.3;margin:12px 0 16px;overflow-wrap:anywhere}h3{font-size:14px;line-height:22px;margin:0 0 8px}
p{font-size:14px;line-height:1.55;white-space:pre-line}.description{margin:0 0 18px}.empty{color:#484949}
section{border-top:1px solid #d0d2d2;margin-top:20px;padding-top:16px}.section-head{display:flex;align-items:baseline;justify-content:space-between;gap:8px}.section-head label{font-size:12px;white-space:nowrap;color:#484949;display:flex;align-items:center;gap:4px}
pre{font:13px/1.6 ui-monospace,SFMono-Regular,monospace;white-space:pre-wrap;overflow-wrap:anywhere;margin:0}
.source{margin-top:24px;font-size:13px}.source summary{cursor:pointer;color:#484949}.source summary span{font-size:11px;margin-left:4px}.source pre{margin-top:12px}
@media(max-width:760px){aside{border-left:0;border-top:1px solid #b1b4b6;padding:12px 18px}h2{margin:6px 0 12px}}
</style>
