<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { SvelteFlow, useSvelteFlow } from '@xyflow/svelte';
  import WorkflowNodeComponent from './WorkflowNode.svelte';
  import Junction from './Junction.svelte';
  import RoutedEdge from './RoutedEdge.svelte';
  import Inspector from './Inspector.svelte';
  import Peek from './Peek.svelte';
  import { layoutWorkflow } from './layout';
  import type { SourcePage, ViewNode, WorkflowNode, WorkflowEdge } from './types';
  let {page}: {page:SourcePage}=$props();
  let nodes=$state.raw<WorkflowNode[]>([]);
  let edges=$state.raw<WorkflowEdge[]>([]);
  let expanded=new Set<string>();
  let selected=$state<ViewNode>();
  let peek=$state<{node:ViewNode;rect:DOMRect}>();
  let error=$state('');
  let busy=$state(false);
  let canvas:HTMLDivElement;
  let revision=0;
  const textCanvas = document.createElement("canvas");
  const textContext = textCanvas.getContext("2d")!;
  textContext.font = "550 15px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
  const flow=useSvelteFlow<WorkflowNode,WorkflowEdge>();
  const nodeTypes={workflow:WorkflowNodeComponent,junction:Junction};
  const edgeTypes={routed:RoutedEdge};
  function position(id:string,list:WorkflowNode[]) {
    const n=list.find(n=>n.id===id);
    if (!n) return;
    let x=n.position.x+n.data.width/2,y=n.position.y+20;
    let parent=n.parentId;
    while(parent){const p=list.find(n=>n.id===parent);if(!p)break;x+=p.position.x;y+=p.position.y;parent=p.parentId;}
    return {x,y};
  }
  async function updateSelection(node?:ViewNode) {
    selected=node;peek=undefined;
    nodes=nodes.map(n=>({...n,data:{...n.data,chosen:!!node && (n.data.sourceNode?.id===node.id || !!n.data.members?.some(m=>m.id===node.id)), selectedId:node?.id}}));
    if(node){
      await tick();
      const target=nodes.find(n=>n.data.sourceNode?.id===node.id || n.data.members?.some(m=>m.id===node.id));
      const point=target && position(target.id,nodes);
      if(point && target){
        const view=flow.getViewport();
        const screenX=point.x*view.zoom+view.x;
        const margin=Math.min(canvas.clientWidth/2,target.data.width*view.zoom/2+24);
        const wanted=Math.max(margin,Math.min(canvas.clientWidth-margin,screenX));
        if(wanted!==screenX)await flow.setViewport({...view,x:view.x+wanted-screenX});
      }
    }
  }
  async function activate(id:string) {
    if(busy)return;
    const n=nodes.find(n=>n.id===id);if(!n)return;
    peek=undefined;
    if(n.data.expandable){
      const before=position(id,nodes),viewport=flow.getViewport();
      if(expanded.has(id))expanded.delete(id);else expanded.add(id);
      await relayout();
      const after=position(id,nodes);
      if(before&&after)await flow.setViewport({...viewport,x:viewport.x+(before.x-after.x)*viewport.zoom,y:viewport.y+(before.y-after.y)*viewport.zoom});
    } else if(n.data.sourceNode) updateSelection(n.data.sourceNode);
  }
  async function relayout(){
    const current=++revision;busy=true;
    try{
      const result=await layoutWorkflow(page,expanded,(label)=>textContext.measureText(label).width);
      if(current!==revision)return;
      nodes=result.nodes.map(n=>({...n,selectable:false,focusable:false,data:{...n.data,chosen:!!selected && (selected.id===n.data.sourceNode?.id || !!n.data.members?.some(m=>m.id===selected?.id)), selectedId:selected?.id,onactivate:activate,onselect: updateSelection,onpeek:(id:string,rect:DOMRect)=>{const n=nodes.find(n=>n.id===id);const source=n?.data.sourceNode;if(source && !n?.data.members)peek={node:{...source,title:source.title || n?.data.label},rect}},onunpeek:()=>peek=undefined}}));
      const headerMasks=nodes.filter(n=>n.data.kind==='container').map(n=>{
        const p=position(n.id,nodes)!;
        return {x:p.x-n.data.width/2+1,y:p.y-19,width:n.data.width-2,height:38};
      });
      edges=result.edges.map(e=>({...e,data:{...e.data!,headerMasks}}));
      await tick();
    }catch(e){error=String(e)}finally{if(current===revision)busy=false}
  }
  function top(){
    peek=undefined;
    const roots=nodes.filter(n=>!n.parentId);
    if(!roots.length)return;
    const left=Math.min(...roots.map(n=>n.position.x)),right=Math.max(...roots.map(n=>n.position.x+n.data.width));
    flow.setViewport({x:(canvas.clientWidth-(right-left))/2-left,y:24-Math.min(...roots.map(n=>n.position.y)),zoom:1});
  }
  async function fit(){peek=undefined;await flow.fitView({padding:0.1,maxZoom:1,minZoom:.35,duration:150});}
  onMount(()=>{document.title=page.name+' · Workflow';relayout();return()=>{revision++}});
</script>
<svelte:window onkeydown={(e)=>{if(e.key==='Escape'){peek=undefined;updateSelection()}}}/>
<div class="viewer">
  <header class="toolbar"><h1>{page.name}</h1><div class="tools"><button onclick={top}>100%</button><button onclick={fit}>Fit view</button></div></header>
  <div class="workspace" class:inspecting={!!selected}>
    <div class="canvas" bind:this={canvas}>
      {#if error}<p role="alert">{error}</p>{/if}
      {#if nodes.length}
        <SvelteFlow bind:nodes bind:edges {nodeTypes} {edgeTypes} nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} nodesFocusable={false} edgesFocusable={false} zoomOnScroll={false} panOnScroll={true} minZoom={.35} maxZoom={1.8} oninit={top} onmovestart={()=>peek=undefined} preventScrolling={true} deleteKey={null}>
        </SvelteFlow>
      {:else if !error}<p class="loading">Preparing workflow…</p>{/if}
    </div>
    {#if selected}<div class="inspector"><Inspector node={selected} onclose={()=>updateSelection()}/></div>{/if}
  </div>
  {#if peek}<Peek node={peek.node} rect={peek.rect}/>{/if}
</div>
<style>
.viewer{height:100%;display:flex;flex-direction:column;overflow:hidden}.toolbar{height:58px;flex:none;padding:0 24px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #b1b4b6;background:white;gap:16px}.toolbar h1{font-size:21px;line-height:1.2;margin:0;white-space:nowrap;text-overflow:ellipsis;overflow:hidden}.tools{display:flex;gap:6px;flex:none}.tools button{font-size:13px;min-height:32px;padding:5px 10px;background:#fff;border:1px solid #b1b4b6;border-radius:4px;cursor:pointer}.tools button:hover{background:#f3f8fc}.workspace{display:flex;min-height:0;flex:1}.canvas{position:relative;min-width:0;flex:1;height:100%;}.inspector{width:360px;flex:none;min-height:0}.loading{margin:28px;color:#484949;font-size:14px}
@media(max-width:760px){.toolbar{padding:0 16px;height:52px}.toolbar h1{font-size:19px}.workspace{flex-direction:column}.inspector{width:100%;height:38%;flex:none}.inspecting .canvas{height:62%}}
</style>
