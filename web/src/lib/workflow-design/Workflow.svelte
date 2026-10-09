<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { SvelteFlow, useSvelteFlow } from '@xyflow/svelte';
  import WorkflowNodeComponent from './WorkflowNode.svelte';
  import Junction from './Junction.svelte';
  import RoutedEdge from './RoutedEdge.svelte';
  import Inspector from './Inspector.svelte';
  import Peek from './Peek.svelte';
  import { layoutWorkflow } from './layout';
  import { indexSource, normalizeView, type ViewState } from './view-state';
  import type { SourcePage, ViewNode, WorkflowNode, WorkflowEdge } from './types';

  let {page, initialView, oncommit, title=false}: {
    page:SourcePage; initialView?:ViewState; title?:boolean;
    oncommit?:(view:ViewState, replace:boolean)=>Promise<void>;
  }=$props();
  let source:SourcePage;
  let view=$state.raw<ViewState>({open:[],details:[],types:false});
  let nodes=$state.raw<WorkflowNode[]>([]);
  let edges=$state.raw<WorkflowEdge[]>([]);
  let selected=$state<ViewNode>();
  let peek=$state<{node:ViewNode;rect:DOMRect}>();
  let error=$state('');
  let busy=$state(false);
  let canvas:HTMLDivElement;
  let revision=0;
  let renderedSource:SourcePage | undefined;
  let renderedOpen='';
  let pendingAnchor:{id:string;point:{x:number;y:number};camera:{x:number;y:number;zoom:number}} | undefined;
  let restoring=false;
  let textContext:CanvasRenderingContext2D;
  function measure(label:string){
    if(!textContext){textContext=document.createElement('canvas').getContext('2d')!;textContext.font='550 15px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif';}
    return textContext.measureText(label).width;
  }
  const flow=useSvelteFlow<WorkflowNode,WorkflowEdge>();
  const nodeTypes={workflow:WorkflowNodeComponent,junction:Junction};
  const edgeTypes={routed:RoutedEdge};

  function position(id:string,list:WorkflowNode[]) {
    const n=list.find(n=>n.id===id); if(!n)return;
    let x=n.position.x+n.data.width/2,y=n.position.y+20,parent=n.parentId;
    while(parent){const p=list.find(n=>n.id===parent);if(!p)break;x+=p.position.x;y+=p.position.y;parent=p.parentId;}
    return {x,y};
  }
  async function commit(next:ViewState,replace=true) {
    const valid=normalizeView(next,source);
    if(oncommit) await oncommit(valid,replace);
    else await show(source,valid);
  }
  function choose(node?:ViewNode) {
    peek=undefined;
    return commit({...view,selected:node?.key,details:[]},false);
  }
  function decorate(list:WorkflowNode[]) {
    return list.map(n=>({...n,selectable:false,focusable:false,data:{...n.data,
      chosen:!!selected && (selected.key===n.data.sourceNode?.key || !!n.data.members?.some(m=>m.key===selected?.key)),
      selectedId:selected?.id,onactivate:activate,onselect:choose,
      onpeek:(id:string,rect:DOMRect,focus=false)=>{
        const n=nodes.find(n=>n.id===id);const source=n?.data.sourceNode;
        if(focus && canvas){const bounds=canvas.getBoundingClientRect(),point=position(id,nodes),camera=flow.getViewport();
          // Native focus may temporarily scroll a transformed ancestor. Use
          // graph coordinates so that scroll cannot disguise an offscreen node.
          const x=point && camera.x+point.x*camera.zoom,y=point && camera.y+point.y*camera.zoom;
          if(x!==undefined && y!==undefined && (x<20 || x>bounds.width-20 || y<20 || y>bounds.height-20)){
            peek=undefined;
            view={...view,camera:{...camera,x:camera.x+bounds.width/2-x,y:camera.y+bounds.height/2-y}};
            void commit(view);return;
          }
        }
        if(source && !n?.data.members)peek={node:{...source,title:source.title || n?.data.label},rect};
      },
      onunpeek:()=>peek=undefined}}));
  }
  async function activate(id:string) {
    if(busy)return;
    const n=nodes.find(n=>n.id===id);if(!n)return;
    peek=undefined;
    if(n.data.expandable){
      const index=indexSource(source);
      const key=[...index.expansionIDs].find(([,graphID])=>graphID===id)?.[0];
      if(!key)return;
      const point=position(id,nodes);
      if(point)pendingAnchor={id,point,camera:flow.getViewport()};
      const open=view.open.includes(key)?view.open.filter(k=>k!==key):[...view.open,key];
      await commit({...view,open});
    } else if(n.data.sourceNode) await choose(n.data.sourceNode);
  }
  // Kit calls this after navigation or a fresh source snapshot. It is also the
  // standalone renderer's entry point. URL commits happen only in event handlers.
  export async function show(nextPage:SourcePage,next:ViewState) {
    const current=++revision;
    source=nextPage;
    view=normalizeView(next,nextPage);
    const index=indexSource(nextPage);
    selected=view.selected?index.selected.get(view.selected):undefined;
    peek=undefined;
    const openKey=view.open.join('|');
    if(renderedSource===nextPage && renderedOpen===openKey){
      nodes=decorate(nodes);busy=false;
      await tick();
      await restoreCamera();
      return;
    }
    busy=true;
    try {
      const expanded=new Set(view.open.map(k=>index.expansionIDs.get(k)).filter((k):k is string=>!!k));
      const result=await layoutWorkflow(nextPage,expanded,measure);
      if(current!==revision)return;
      nodes=decorate(result.nodes);
      const headerMasks=nodes.filter(n=>n.data.kind==='container').map(n=>{
        const p=position(n.id,nodes)!;
        return {x:p.x-n.data.width/2+1,y:p.y-19,width:n.data.width-2,height:38};
      });
      edges=result.edges.map(e=>({...e,data:{...e.data!,headerMasks}}));
      renderedSource=nextPage;renderedOpen=openKey;error='';
      await tick();
      const anchor=pendingAnchor;pendingAnchor=undefined;
      const after=anchor && position(anchor.id,nodes);
      if(anchor && after){
        const camera={...anchor.camera,x:anchor.camera.x+(anchor.point.x-after.x)*anchor.camera.zoom,y:anchor.camera.y+(anchor.point.y-after.y)*anchor.camera.zoom};
        await commit({...view,camera});
      }else await restoreCamera();
    }catch(e){error=String(e)}finally{if(current===revision)busy=false}
  }
  function topCamera(){
    const roots=nodes.filter(n=>!n.parentId);
    if(!roots.length)return {x:0,y:0,zoom:1};
    const left=Math.min(...roots.map(n=>n.position.x)),right=Math.max(...roots.map(n=>n.position.x+n.data.width));
    return {x:(canvas.clientWidth-(right-left))/2-left,y:24-Math.min(...roots.map(n=>n.position.y)),zoom:1};
  }
  async function restoreCamera(){
    restoring=true;
    try{await flow.setViewport(view.camera ?? topCamera())}finally{restoring=false}
  }
  async function fit(){
    peek=undefined;restoring=true;
    try{await flow.fitView({padding:.1,maxZoom:1,minZoom:.35});await commit({...view,camera:flow.getViewport()})}finally{restoring=false}
  }
  function detail(path:string,open:boolean){
    commit({...view,details:open?[...view.details,path]:view.details.filter(p=>p!==path)});
  }
  onMount(()=>{show(page,initialView ?? {open:[],details:[],types:false});return()=>{revision++}});
</script>
<svelte:window onkeydown={(e)=>{if(e.key==='Escape'){peek=undefined;choose()}}}/>
<div class="viewer">
  <header class="toolbar">{#if title}<h1>{page.name}</h1>{:else}<span class="sr-only">Workflow diagram controls</span>{/if}<div class="tools"><button onclick={()=>commit({...view,camera:topCamera()})}>100%</button><button onclick={fit}>Fit view</button></div></header>
  <div class="workspace" class:inspecting={!!selected}>
    <div class="canvas" bind:this={canvas}>
      {#if error}<p role="alert">{error}</p>{/if}
      {#if nodes.length}
        <SvelteFlow bind:nodes bind:edges {nodeTypes} {edgeTypes} nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} nodesFocusable={false} edgesFocusable={false} zoomOnScroll={false} panOnScroll={true} minZoom={.35} maxZoom={1.8} oninit={restoreCamera} onmovestart={()=>peek=undefined} onmoveend={(event)=>{if(event && !busy && !restoring)commit({...view,camera:flow.getViewport()})}} preventScrolling={true} deleteKey={null} />
      {:else if !error}<p class="loading">Preparing workflow…</p>{/if}
    </div>
    {#if selected}<div class="inspector"><Inspector node={selected} onclose={()=>choose()} goTypes={view.types} ontypes={(types)=>commit({...view,types})} openPaths={view.details} ondetail={detail}/></div>{/if}
  </div>
  {#if peek}<Peek node={peek.node} rect={peek.rect}/>{/if}
</div>
<style>
.viewer{height:100%;display:flex;flex-direction:column;overflow:hidden}.toolbar{height:58px;flex:none;padding:0 24px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #b1b4b6;background:white;gap:16px}.toolbar h1{font-size:21px;line-height:1.2;margin:0;white-space:nowrap;text-overflow:ellipsis;overflow:hidden}.tools{display:flex;gap:6px;flex:none}.tools button{font-size:13px;min-height:32px;padding:5px 10px;background:#fff;border:1px solid #b1b4b6;border-radius:4px;cursor:pointer}.tools button:hover{background:#f3f8fc}.workspace{display:flex;min-height:0;flex:1}.canvas{position:relative;min-width:0;flex:1;height:100%;}.inspector{width:360px;flex:none;min-height:0}.loading{margin:28px;color:#484949;font-size:14px}
@media(max-width:760px){.toolbar{padding:0 16px;height:52px}.toolbar h1{font-size:19px}.workspace{flex-direction:column}.inspector{width:100%;height:38%;flex:none}.inspecting .canvas{height:62%}}
</style>
