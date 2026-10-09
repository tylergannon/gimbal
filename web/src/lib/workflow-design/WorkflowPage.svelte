<script lang="ts">
 import {onMount,untrack} from 'svelte';
 import {dev} from '$app/env';
 import {afterNavigate,goto} from '$app/navigation';
 import {page as route} from '$app/state';
 import '@xyflow/svelte/dist/style.css';
 import type WorkflowCanvas from './WorkflowCanvas.svelte';
 import Guide from './Guide.svelte';
 import StartWorkflow from './generated/StartWorkflow.svelte';
 import {decodeView,encodeView,normalizeView,type ViewState} from './view-state';
 import type {SourcePage} from './types';
 import './viewer.css';
 type Snapshot={page?:SourcePage;revision:number;status:'ready'|'updating'|'error';error?:string};
 let {initial}:{initial:SourcePage}=$props();
 const first=untrack(()=>initial);
 const panels=['workflow','guide','run'] as const;
 let snapshot=$state.raw<Snapshot>({page:first,revision:0,status:'ready'});
 let current=$derived(snapshot.page ?? initial);
 let visibleURL=$derived(route.shallow?.url ?? route.url);
 let view=$derived(decodeView(new URLSearchParams(visibleURL.search),current));
 let panel=$derived(view.panel ?? 'workflow');
 let disconnected=$state(false);
 let viewer=$state<WorkflowCanvas>();
 let Renderer=$state<typeof WorkflowCanvas>();
 function navigatePanel(event:MouseEvent,item:typeof panels[number]){
  if(event.button===0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey){event.preventDefault();void goto(panelURL(item),{shallow:true})}
 }
 function panelURL(next:'workflow'|'guide'|'run'){
  const url=new URL(visibleURL.href);url.search=encodeView({...view,panel:next==='workflow'?undefined:next}).toString();return url.pathname+url.search;
 }
 async function applyURL(requestedURL=new URL(visibleURL.href)){
  const valid=decodeView(requestedURL.searchParams,current);
  const url=new URL(requestedURL.href);url.search=encodeView(valid).toString();
  if(url.href!==requestedURL.href){await goto(url,{shallow:true,replace:true});return}
  await viewer?.show(current,valid);
 }
 async function commit(state:ViewState,replace:boolean){
  const url=new URL(visibleURL.href);url.search=encodeView(normalizeView({...state,panel:view.panel},current)).toString();
  if(url.href===visibleURL.href)await applyURL();else await goto(url,{shallow:true,replace});
 }
 async function accept(next:Snapshot,force=false){
  if(!force && next.revision<=snapshot.revision)return;
  snapshot={...next,page:next.page ?? current};
  if(next.page)await applyURL();
 }
 async function refresh(){
  try{
   // Vite's development-only source transport, not a runtime API.
   const response=await fetch(`/@workflow-source/${encodeURIComponent(initial.name)}`,{cache:'no-store'});
   if(!response.ok)throw new Error('Source unavailable');
   disconnected=false;await accept(await response.json(),true);
  }catch{disconnected=true}
 }
 afterNavigate(()=>{void applyURL()});
 function restoreHistory(){const url=new URL(window.location.href);if(url.pathname===visibleURL.pathname)void applyURL(url)}
 onMount(()=>{
  let active=true;
  void import('./WorkflowCanvas.svelte').then(module=>{if(active)Renderer=module.default});
  if(!dev)return()=>{active=false};
  const update=(next:Snapshot & {name:string})=>{if(next.name===initial.name)void accept(next)};
  const offline=()=>{disconnected=true};const reconnect=()=>{void refresh()};
  import.meta.hot?.on('workflow:source',update);
  import.meta.hot?.on('vite:ws:disconnect',offline);
  import.meta.hot?.on('vite:ws:connect',reconnect);
  void refresh();
  return()=>{active=false;import.meta.hot?.off('workflow:source',update);import.meta.hot?.off('vite:ws:disconnect',offline);import.meta.hot?.off('vite:ws:connect',reconnect)};
 });
</script>
<svelte:window onpopstate={restoreHistory}/>
<svelte:head><title>{current.name} — Gimbal</title></svelte:head>
<section class="workflow-page">
 <header class="page-heading"><h1>{current.name}</h1><p>{current.guide?.summary}</p></header>
 <nav class="views" aria-label="Workflow views">
  {#each panels as item}<a href={panelURL(item)} aria-current={panel===item?'page':undefined} onclick={(event)=>navigatePanel(event,item)}>{item==='workflow'?'Workflow':item==='guide'?'Guide':'Run'}</a>{/each}
 </nav>
 {#if disconnected}<p class="source-status" role="status">Source updates disconnected. Showing the last received version.</p>
 {:else if snapshot.status==='error'}<details class="source-status error"><summary>Showing previous version — source has errors</summary><pre>{snapshot.error}</pre></details>
 {:else if snapshot.status==='updating'}<p class="source-status" role="status">Updating from source…</p>{/if}
 {#if current.diagnostics?.length}<details class="source-status"><summary>Some source could not be represented ({current.diagnostics.length})</summary>{#each current.diagnostics as diagnostic}<p>{diagnostic.message}</p>{/each}</details>{/if}
 <div class="diagram workflow-viewer" hidden={panel!=='workflow'}>
  {#if Renderer}<Renderer bind:this={viewer} page={current} initialView={view} oncommit={commit}/>{:else}<p class="loading" role="status">Preparing workflow…</p>{/if}
 </div>
 {#if panel==='guide'}<div class="reading"><Guide page={current}/></div>
 {:else if panel==='run'}<div class="reading"><h2>Run {current.name}</h2><StartWorkflow name={initial.name} guide={current.guide!}/></div>{/if}
</section>
<style>
 .workflow-page{display:flex;flex-direction:column;height:calc(100dvh - 54px);min-height:420px;color:#0b0c0c;background:#fff}
 .page-heading{padding:14px 24px 10px;flex:none}.page-heading h1{font-size:22px;line-height:1.3;font-weight:650;margin:0}.page-heading p{font-size:14px;line-height:1.45;margin:4px 0 0;max-width:65ch}
 .views{display:flex;gap:20px;padding:0 24px;border-bottom:1px solid #a6acb0;flex:none}.views a{display:block;font-size:14px;padding:9px 0;color:#333b41;text-decoration:none;border-bottom:3px solid transparent}.views a[aria-current]{border-color:#1a65a6;color:#124d80;font-weight:650}
 .diagram{flex:1;min-height:0}.diagram[hidden]{display:none}.reading{overflow:auto;padding:24px;flex:1;min-height:0}
 .source-status{flex:none;margin:0;padding:8px 24px;font-size:13px;background:#fff4cc;color:#403100;border-bottom:1px solid #8a6a00}.source-status pre{white-space:pre-wrap;max-height:150px;overflow:auto}.error{background:#fff0ec;color:#792510}
 .views a:focus-visible{outline:3px solid #0b0c0c;outline-offset:2px;box-shadow:0 0 0 5px #ffdd00}
 @media(max-width:760px){.page-heading{padding:10px 16px}.page-heading p{font-size:13px}.views{padding:0 16px}.reading{padding:20px 16px}}
</style>
