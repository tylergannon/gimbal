<script lang="ts">
  import { onMount } from 'svelte';
  import { afterNavigate, goto } from '$app/navigation';
  import { page as route } from '$app/state';
  import { SvelteFlowProvider } from '@xyflow/svelte';
  import Workflow from './Workflow.svelte';
  import { decodeView, encodeView, normalizeView, type ViewState } from './view-state';
  import type { SourcePage } from './types';

  type Snapshot={page?:SourcePage;revision:number;status:'ready'|'updating'|'error';error?:string};
  let {name,initial}:{name:string;initial:Snapshot}=$props();
  let received=$state.raw<Snapshot>();
  let snapshot=$derived(received ?? initial);
  let disconnected=$state(false);
  let viewer=$state<Workflow>();
  let visibleURL=$derived(route.shallow?.url ?? route.url);
  let initialView=$derived(snapshot.page?decodeView(new URLSearchParams(visibleURL.search),snapshot.page):undefined);

  async function applyURL(requestedURL=new URL(visibleURL.href)) {
    if(!snapshot.page)return;
    const valid=decodeView(requestedURL.searchParams,snapshot.page);
    const url=new URL(requestedURL.href);
    url.search=encodeView(valid).toString();
    if(url.href!==requestedURL.href){
      await goto(url,{shallow:true,replace:true});
      return;
    }
    await viewer?.show(snapshot.page,valid);
  }
  async function commit(state:ViewState,replace:boolean) {
    if(!snapshot.page)return;
    const url=new URL(visibleURL.href);
    url.search=encodeView(normalizeView(state,snapshot.page)).toString();
    if(url.href===visibleURL.href)await applyURL();
    else await goto(url,{shallow:true,replace});
  }
  async function accept(next:Snapshot,force=false){
    if(!force && next.revision<=snapshot.revision)return;
    received=next;
    // Errors carry the last successful source. Never normalize against an
    // incomplete edit or erase valid URL choices while source is unavailable.
    if(next.page)await applyURL();
  }
  async function refresh(){
    try{
      const response=await fetch(`/__workflow/${encodeURIComponent(name)}`);
      if(!response.ok)throw new Error('Preview source unavailable');
      disconnected=false;
      await accept(await response.json(),true);
    }catch{disconnected=true}
  }
  afterNavigate(()=>{void applyURL()});
  function restoreHistory(){
    // Kit 3 shallow popstate updates page but skips afterNavigate. Restore
    // directly from the browser URL; ordinary route navigation uses Kit above.
    const url=new URL(window.location.href);
    if(url.pathname===visibleURL.pathname)void applyURL(url);
  }
  onMount(()=>{
    const update=(data:Snapshot & {name:string})=>{if(data.name===name)void accept(data)};
    const offline=()=>{disconnected=true};
    const reconnect=()=>{void refresh()};
    import.meta.hot?.on('workflow:source',update);
    import.meta.hot?.on('vite:ws:disconnect',offline);
    import.meta.hot?.on('vite:ws:connect',reconnect);
    void refresh();
    return()=>{
      import.meta.hot?.off('workflow:source',update);
      import.meta.hot?.off('vite:ws:disconnect',offline);
      import.meta.hot?.off('vite:ws:connect',reconnect);
    };
  });
</script>

<svelte:window onpopstate={restoreHistory}/>
<div class="live-preview">
  {#if disconnected}
    <p class="status" role="status">Preview disconnected. Showing the last received version.</p>
  {:else if snapshot.status==='error'}
    <details class="status error"><summary>{snapshot.page?'Showing previous version — source has errors':'Source could not be read'}</summary><pre>{snapshot.error}</pre></details>
  {:else if snapshot.status==='updating'}
    <p class="status" role="status">Updating from source…</p>
  {/if}
  {#if snapshot.page?.diagnostics?.length}
    <details class="status"><summary>Some source could not be represented ({snapshot.page.diagnostics.length})</summary>
      {#each snapshot.page.diagnostics as diagnostic}<p>{diagnostic.message ?? JSON.stringify(diagnostic)}</p>{/each}
    </details>
  {/if}
  <div class="flow">
    {#if snapshot.page}
      <SvelteFlowProvider><Workflow bind:this={viewer} page={snapshot.page} {initialView} oncommit={commit}/></SvelteFlowProvider>
    {:else if snapshot.status!=='error'}<p class="loading">Reading workflow source…</p>{/if}
  </div>
</div>
<style>
  .live-preview{height:100%;display:flex;flex-direction:column}.flow{flex:1;min-height:0}.status{flex:none;margin:0;padding:8px 20px;background:#fff4cc;border-bottom:1px solid #8a6a00;font-size:13px;color:#403100}.status.error{background:#fff0ec;color:#792510;border-color:#a23920}.status summary{cursor:pointer}.status pre{white-space:pre-wrap;max-height:180px;overflow:auto;font-size:12px}.loading{padding:24px}
</style>
