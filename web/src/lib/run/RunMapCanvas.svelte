<script lang="ts">
  import { tick, untrack } from "svelte";
  import { SvelteFlow, useSvelteFlow } from "@xyflow/svelte";
  import WorkflowNodeComponent from "../workflow-design/WorkflowNode.svelte";
  import Junction from "../workflow-design/Junction.svelte";
  import RoutedEdge from "../workflow-design/RoutedEdge.svelte";
  import { layoutWorkflow } from "../workflow-design/layout.js";
  import type { SourcePage, ViewNode, WorkflowEdge, WorkflowNode } from "../workflow-design/types.js";
  import type { RunSnapshot } from "../observation/index.js";
  import type { Graph } from "../workflow/types.js";
  import type { MapSelection } from "./types.js";
  import { bindRun, selectionKeyOf, type WatcherMark } from "./runSource.js";
  import { selectedInstancesFor } from "./selection.js";

  let {
    source,
    graph,
    snapshot,
    selected,
    inspected,
    reveal,
    onselect,
    oninspect,
    onopen,
  }: {
    source: SourcePage;
    graph: Graph;
    snapshot: RunSnapshot;
    selected?: MapSelection;
    inspected?: ViewNode;
    reveal?: { request: number; selection: MapSelection };
    onselect?: (selection: MapSelection) => void;
    oninspect?: (node: ViewNode) => void;
    onopen?: (selection: MapSelection) => void;
  } = $props();

  const flow = useSvelteFlow<WorkflowNode, WorkflowEdge>();
  const nodeTypes = { workflow: WorkflowNodeComponent, junction: Junction };
  const edgeTypes = { routed: RoutedEdge };

  let selectedInstances = $state<Record<string, string>>({});
  /** The viewer's own open/closed choices, over the run's default. */
  let toggled = $state<Record<string, boolean>>({});
  let laid = $state.raw<{ nodes: WorkflowNode[]; edges: WorkflowEdge[] }>();
  let nodes = $state.raw<WorkflowNode[]>([]);
  let edges = $state.raw<WorkflowEdge[]>([]);
  let error = $state("");
  let canvas: HTMLDivElement | undefined;
  let placed = false;
  let revealed = 0;
  let pendingReveal: string | undefined;

  const binding = $derived(bindRun(source, graph, snapshot, selectedInstances));
  const selectedKey = $derived(selectionKeyOf(selected));

  /** Parent container of each designer node, for opening a path to it. */
  const parents = $derived.by(() => {
    const map = new Map<string, string>();
    const visit = (list: ViewNode[], parent?: string) => {
      for (const node of list) {
        if (parent) map.set(node.id, parent);
        for (const lane of node.children ?? []) visit(lane, node.id);
      }
    };
    visit(source.body);
    return map;
  });

  // Containers open where the run has been, closed where it has not; the
  // viewer's own toggles win. Only the membership string drives layout.
  const expandedKey = $derived.by(() => {
    const open = new Set(binding.active);
    for (const [id, value] of Object.entries(toggled)) {
      if (value) open.add(id);
      else open.delete(id);
    }
    return [...open].sort().join("|");
  });

  let textContext: CanvasRenderingContext2D | undefined;
  function measure(label: string) {
    if (!textContext) {
      textContext = document.createElement("canvas").getContext("2d")!;
      textContext.font = "550 15px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
    }
    // Room for the status pip and the turn or instance note.
    return textContext.measureText(label).width + 48;
  }

  let layoutRevision = 0;
  $effect(() => {
    const page = source;
    const expanded = new Set(expandedKey ? expandedKey.split("|") : []);
    const current = ++layoutRevision;
    layoutWorkflow(page, expanded, measure)
      .then((result) => {
        if (current !== layoutRevision) return;
        laid = result;
        error = "";
      })
      .catch((reason) => {
        if (current === layoutRevision) error = String(reason);
      });
  });

  function absolute(id: string, list: WorkflowNode[]) {
    const node = list.find((n) => n.id === id);
    if (!node) return undefined;
    let x = node.position.x + node.data.width / 2;
    let y = node.position.y + Math.min(node.data.height, 40) / 2;
    let parent = node.parentId;
    while (parent) {
      const p = list.find((n) => n.id === parent);
      if (!p) break;
      x += p.position.x;
      y += p.position.y;
      parent = p.parentId;
    }
    return { x, y };
  }

  const viewNodeOf = (id: string) => nodes.find((n) => n.id === id)?.data.sourceNode;

  function pick(id: string) {
    const node = nodes.find((n) => n.id === id);
    const view = node?.data.sourceNode;
    if (!node || !view) return;
    const mark = id === view.id ? binding.marks.get(view.id) : undefined;
    if (mark?.selection) onselect?.(mark.selection);
    else if (node.data.expandable && node.data.kind !== "container" && !node.data.expanded) toggle(id);
    else oninspect?.(view);
  }

  function toggle(id: string) {
    const node = nodes.find((n) => n.id === id);
    if (!node) return;
    toggled = { ...toggled, [id]: !node.data.expanded };
  }

  function decorate(list: WorkflowNode[]) {
    return list.map((n) => {
      const view = n.data.sourceNode;
      const mark = view && n.id === view.id ? binding.marks.get(view.id) : undefined;
      return {
        ...n,
        selectable: false,
        focusable: false,
        data: {
          ...n.data,
          run: mark,
          chosen: mark?.selectionKey
            ? mark.selectionKey === selectedKey
            : !!inspected && !!view && inspected.id === view.id && n.id === view.id,
          chosenKey: selectedKey,
          selectedId: inspected?.id,
          onpick: pick,
          onactivate: toggle,
          onopen: (id: string) => {
            const own = viewNodeOf(id);
            const selection = own ? binding.marks.get(own.id)?.selection : undefined;
            if (selection?.kind === "node") onopen?.(selection);
          },
          onselect: (member: ViewNode) => oninspect?.(member),
          oninstance: (group: string, key: string) => {
            selectedInstances = { ...selectedInstances, [group]: key };
          },
          onwatcher: (watcher: WatcherMark) => onselect?.(watcher.selection),
        },
      } as WorkflowNode;
    });
  }

  $effect(() => {
    if (!laid) return;
    const decorated = decorate(laid.nodes);
    const masks = decorated
      .filter((n) => n.data.kind === "container")
      .map((n) => {
        const p = absolute(n.id, decorated)!;
        return { x: p.x - n.data.width / 2 + 1, y: p.y - 19, width: n.data.width - 2, height: 38 };
      });
    nodes = decorated;
    edges = laid.edges.map((e) => ({ ...e, data: { ...e.data!, headerMasks: masks } }));
    untrack(() => void settleCamera());
  });

  /** The designer node a selection key belongs to. */
  function nodeForKey(key: string) {
    for (const [id, mark] of binding.marks) {
      if (mark.selectionKey === key || mark.watchers.some((w) => w.selectionKey === key)) return id;
    }
    return undefined;
  }

  async function center(id: string) {
    const point = absolute(id, nodes);
    if (point) await flow.setCenter(point.x, point.y, { zoom: 1, duration: 200 });
  }

  async function settleCamera() {
    await tick();
    if (pendingReveal) {
      const id = nodeForKey(pendingReveal);
      if (id && nodes.some((n) => n.id === id)) {
        pendingReveal = undefined;
        placed = true;
        await center(id);
        return;
      }
    }
    if (placed || !canvas || nodes.length === 0) return;
    placed = true;
    const live = [...binding.marks].find(
      ([id, mark]) =>
        (mark.state === "running" || mark.state === "waiting") &&
        !nodes.find((n) => n.id === id)?.data.expandable,
    )?.[0];
    if (live && nodes.some((n) => n.id === live)) await center(live);
    else await top();
  }

  async function top() {
    const roots = nodes.filter((n) => !n.parentId);
    if (!canvas || roots.length === 0) return;
    const left = Math.min(...roots.map((n) => n.position.x));
    const right = Math.max(...roots.map((n) => n.position.x + n.data.width));
    await flow.setViewport({
      x: (canvas.clientWidth - (right - left)) / 2 - left,
      y: 24 - Math.min(...roots.map((n) => n.position.y)),
      zoom: 1,
    });
  }

  // A reveal (search, "current") shows the instances that hold the
  // selection, opens the containers above it and centers it.
  $effect(() => {
    if (!reveal || reveal.request === revealed) return;
    revealed = reveal.request;
    const target = reveal.selection;
    untrack(() => {
      selectedInstances = { ...selectedInstances, ...selectedInstancesFor(target.scope.key) };
      const key = selectionKeyOf(target);
      if (!key) return;
      pendingReveal = key;
      const id = nodeForKey(key);
      if (id) {
        const open: Record<string, boolean> = {};
        for (let parent = parents.get(id); parent; parent = parents.get(parent)) open[parent] = true;
        toggled = { ...toggled, ...open };
      }
      void settleCamera();
    });
  });
</script>

<div class="run-map" role="region" aria-label={`${source.name} workflow map`} bind:this={canvas}>
  {#if error}<p role="alert" class="notice">{error}</p>{/if}
  {#if nodes.length}
    <SvelteFlow
      bind:nodes
      bind:edges
      {nodeTypes}
      {edgeTypes}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable={false}
      nodesFocusable={false}
      edgesFocusable={false}
      zoomOnScroll={false}
      panOnScroll={true}
      minZoom={0.3}
      maxZoom={1.8}
      preventScrolling={true}
      deleteKey={null}
      proOptions={{ hideAttribution: true }}
    />
    <div class="tools">
      <button type="button" onclick={top}>Top</button>
      <button type="button" onclick={() => flow.fitView({ padding: 0.1, maxZoom: 1, minZoom: 0.3 })}>Fit</button>
    </div>
  {:else if !error}<p class="notice">Drawing the workflow…</p>{/if}
</div>

<style>
  .run-map {
    position: relative;
    min-width: 0;
    flex: 1;
    height: 100%;
    color: var(--graph-ink);
    background: var(--background);
  }
  .run-map :global(.svelte-flow) {
    --xy-background-color: var(--background);
    --xy-edge-stroke: var(--graph-ink);
    --xy-node-boxshadow-hover: none;
    --xy-node-boxshadow-selected: none;
  }
  .run-map :global(.svelte-flow__node-workflow),
  .run-map :global(.svelte-flow__node-junction) {
    border: 0;
    background: transparent;
    padding: 0;
  }
  .run-map :global(.svelte-flow__edge-labels) { z-index: 1; }
  .run-map :global(.svelte-flow__pane) { cursor: grab; }
  .tools {
    position: absolute;
    top: 12px;
    right: 12px;
    z-index: 5;
    display: flex;
    gap: 6px;
  }
  .tools button {
    min-height: 30px;
    padding: 4px 10px;
    font: inherit;
    font-size: 13px;
    color: var(--foreground);
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }
  .tools button:hover { background: var(--accent); }
  .notice { margin: 24px; color: var(--muted-foreground); font-size: 14px; }
</style>
