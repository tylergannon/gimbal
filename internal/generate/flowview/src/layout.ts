import ELK from 'elkjs/lib/elk.bundled.js';
import type { ElkExtendedEdge, ElkNode, ElkPort } from 'elkjs/lib/elk-api';
import type { Point, SourcePage, ViewNode, WorkflowData, WorkflowEdge, WorkflowNode } from './types';

const elk = new ELK();
const BOX_WIDTH = 204;
const BOX_HEIGHT = 40;
const DIAMOND_WIDTH = 204;
const DIAMOND_HEIGHT = 76;

type Endpoint = { node: string; port: string };
type Fragment = { entry: Endpoint; exit?: Endpoint; alternative?: Endpoint };
type NodeRecord = { node: ElkNode; data: WorkflowData; parent?: string };

/** Layout the currently disclosed source graph; closed scopes remain one node. */
export async function layoutWorkflow(page: SourcePage, expanded: Set<string>, measureText: (label: string) => number = label => label.length * 8): Promise<{ nodes: WorkflowNode[]; edges: WorkflowEdge[] }> {
  const records = new Map<string, NodeRecord>();
  const edgeEndpoints = new Map<string, { source: Endpoint; target: Endpoint }>();
  let edgeIndex = 0;
  const graph: ElkNode = {
    id: 'workflow-root', children: [], edges: [],
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'DOWN',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
      'elk.padding': '[top=24,left=24,bottom=24,right=24]',
      'elk.spacing.nodeNode': '32',
      'elk.layered.spacing.nodeNodeBetweenLayers': '24',
      'elk.spacing.edgeNode': '18',
      'elk.layered.spacing.edgeNodeBetweenLayers': '18',
      'elk.layered.considerModelOrder.strategy': 'NODES_AND_EDGES',
      'elk.layered.crossingMinimization.forceNodeModelOrder': 'true',
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      'elk.layered.unnecessaryBendpoints': 'true',
    },
  };
  function endpoint(id: string, side: 'in' | 'out'): Endpoint { return { node: id, port: `${id}-${side}` }; }
  function addNode(parent: ElkNode, id: string, data: WorkflowData): ElkNode {
    if (data.kind !== 'junction' && data.kind !== 'Return') {
      data.width = Math.max(BOX_WIDTH, Math.min(340, measureText(data.label) + (data.expandable ? 96 : 64)));
    }
    const ports: ElkPort[] = [
      { id: `${id}-in`, width: 0, height: 0, layoutOptions: { 'elk.port.side': 'NORTH' } },
      { id: `${id}-out`, width: 0, height: 0, layoutOptions: { 'elk.port.side': 'SOUTH' } },
    ];
    const node: ElkNode = {
      id, width: data.width, height: data.height, ports,
      layoutOptions: { 'elk.portConstraints': 'FIXED_SIDE' },
    };
    (parent.children ??= []).push(node);
    records.set(id, { node, data, parent: parent === graph ? undefined : parent.id });
    return node;
  }
  function dot(parent: ElkNode, id: string, kind = 'junction'): Fragment {
    addNode(parent, id, { label: '', kind, width: 8, height: 8 });
    return { entry: endpoint(id, 'in'), exit: endpoint(id, 'out') };
  }
  function edge(source: Endpoint, target: Endpoint, label = ''): void {
    const id = `flow-${edgeIndex++}`;
    const e: ElkExtendedEdge = { id, sources: [source.port], targets: [target.port] };
    if (label) e.labels = [{
      id: `${id}-label`, text: label, width: Math.min(228, Math.max(30, label.length * 7.3 + 16)), height: 24,
      layoutOptions: { 'elk.edgeLabels.inline': 'true', 'elk.edgeLabels.placement': 'CENTER' },
    }];
    // Cross-scope edges belong at the graph root. ELK splits/routes hierarchy
    // crossings and returns their owning container with the resulting sections.
    graph.edges!.push(e);
    edgeEndpoints.set(id, { source, target });
  }
  function labelFor(n: ViewNode): string {
    if (n.title) return n.title;
    switch (n.kind) {
      case 'Condition': return n.control?.kind === 'switch' || n.control?.code.trimStart().startsWith('switch ') ? 'Choose a path' : 'Condition';
      case 'Repeat': case 'Iterate': case 'PromiseLoop': return 'Loop';
      case 'Generate': return n.session ? `${n.session} · Generate` : 'Generate';
      case 'NewSession': return n.label;
      case 'Fork': return n.label;
      default: return n.label || n.kind;
    }
  }
  function one(parent: ElkNode, n: ViewNode): Fragment {
    const expandable = !!n.children?.length;
    const open = expandable && expanded.has(n.id);
    if (n.kind === 'Condition') return condition(parent, n, open);
    if (open) return container(parent, n);
    addNode(parent, n.id, {
      label: labelFor(n), kind: n.kind, sourceNode: n,
      expanded: false, expandable, width: Math.max(BOX_WIDTH, Math.min(340, labelFor(n).length * 8.2 + 72)), height: BOX_HEIGHT,
    });
    return { entry: endpoint(n.id, 'in'), exit: endpoint(n.id, 'out') };
  }
  function sequence(parent: ElkNode, list: ViewNode[], prefix: string): Fragment {
    let first: Endpoint | undefined;
    let previous: Endpoint | undefined;
    for (let i = 0; i < list.length; i++) {
      const n = list[i];
      let part: Fragment;
      if (n.kind === 'Set' || n.kind === 'SetJSON') {
        const members = [n];
        while (i + 1 < list.length && ['Set', 'SetJSON'].includes(list[i + 1].kind)) members.push(list[++i]);
        const id = `${n.id}-writes`;
        const open = expanded.has(id);
        addNode(parent, id, {
          label: 'Set context', kind: 'writes', members, sourceNode: n, expanded: open,
          expandable: true, width: BOX_WIDTH, height: open ? 44 + 32 * members.length : BOX_HEIGHT,
        });
        part = { entry: endpoint(id, 'in'), exit: endpoint(id, 'out') };
      } else part = one(parent, n);
      first ??= part.entry;
      if (previous) edge(previous, part.entry);
      previous = part.exit;
    }
    if (!first) return dot(parent, `${prefix}-empty`);
    return { entry: first, exit: previous };
  }
  function compoundOptions(node: ElkNode): void {
    // Leaf endpoints inside the scope allow ELK to route real hierarchy
    // crossings, avoiding independently laid-out boxes stitched by hand.
    delete node.width;
    delete node.height;
    delete node.ports;
    node.children = [];
    node.layoutOptions = {
      'elk.padding': '[top=54,left=20,bottom=20,right=20]',
      'elk.spacing.nodeNode': '28',
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      'elk.nodeSize.constraints': 'MINIMUM_SIZE',
      'elk.nodeSize.minimum': `(${records.get(node.id)!.data.width},40)`,
      'elk.layered.spacing.nodeNodeBetweenLayers': '20',
      'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
    };
  }
  function container(parent: ElkNode, n: ViewNode): Fragment {
    const group = addNode(parent, n.id, {
      label: labelFor(n), kind: 'container', sourceNode: n,
      expanded: true, expandable: true, width: BOX_WIDTH, height: BOX_HEIGHT,
    });
    compoundOptions(group);
    const start = dot(group, `${n.id}-entry`);
    const finish = dot(group, `${n.id}-exit`);
    for (const [i, body] of (n.children || []).entries()) {
      const lane = sequence(group, body, `${n.id}-lane-${i}`);
      edge(start.exit!, lane.entry);
      if (lane.exit) edge(lane.exit, finish.entry);
    }
    if (!n.children?.length) edge(start.exit!, finish.entry);
    // Compound sizing with crossing edges can ignore ELK's minimum size.
    // Reserve enough side padding around the widest child for the real header.
    const widestChild=Math.max(BOX_WIDTH,...(group.children||[]).map(child=>records.get(child.id)!.data.width));
    const side=Math.max(20,(records.get(n.id)!.data.width-widestChild)/2);
    group.layoutOptions!['elk.padding']=`[top=54,left=${side},bottom=20,right=${side}]`;
    return { entry: start.entry, exit: finish.exit };
  }
  function condition(parent: ElkNode, n: ViewNode, open: boolean): Fragment {
    const labels = n.branchLabels || [];
    const metadata = n.branches || [];
    const isSwitch = n.control?.kind === 'switch' || n.control?.code.trimStart().startsWith('switch ') || false;
    const branches = n.children || [];
    const defaultIndex = branches.findIndex((_, i) => metadata[i]?.default || labels[i] === 'else / default');
    const decision = (id: string, index: number): Fragment => {
      const branch = metadata[index];
      const sourceNode = index === 0 ? n : {
        ...n, id, key: branch?.key, title: branch?.title, description: branch?.description,
        source: branch?.source || n.source,
        control: { kind: 'if', code: branch?.code || labels[index] || '' },
      };
      const node = addNode(parent, id, {
        label: index === 0 ? labelFor(n) : branch?.title || 'Else if', kind: 'Condition', sourceNode,
        expandable: index === 0, expanded: open, width: DIAMOND_WIDTH, height: DIAMOND_HEIGHT,
      });
      if (open && !isSwitch) {
        node.ports![1].layoutOptions = { 'elk.port.side': 'WEST' };
        node.ports!.push({ id: `${id}-alternative`, width: 0, height: 0, layoutOptions: { 'elk.port.side': 'EAST' } });
      }
      return { entry: endpoint(id, 'in'), exit: endpoint(id, 'out'), alternative: { node: id, port: `${id}-alternative` } };
    };
    const first = decision(n.id, 0);
    if (!open) return first;
    const merge = dot(parent, `${n.id}-merge`);
    let hasContinuation = false;
    const branchBody = (i: number, from: Endpoint, title: string): void => {
      const body = branches[i] || [];
      const exits = n.branchExits?.[i];
      const action = n.control?.actions?.[i]?.trim().split(/\s+/)[0];
      const exitLabel = action === 'return' ? 'Return' : action === 'break' ? 'Break' : action === 'continue' ? 'Continue' : 'Exit';
      if (!body.length) {
        if (exits) {
          const stopID = `${n.id}-exit-${i}`;
          addNode(parent, stopID, { label: exitLabel, kind: 'Return', width: 104, height: 36, sourceNode: n });
          edge(from, endpoint(stopID, 'in'), title);
        } else { edge(from, merge.entry, title); hasContinuation = true; }
        return;
      }
      const path = sequence(parent, body, `${n.id}-branch-${i}`);
      edge(from, path.entry, title);
      if (exits) {
        const stopID = `${n.id}-exit-${i}`;
        addNode(parent, stopID, { label: exitLabel, kind: 'Return', width: 104, height: 36, sourceNode: n });
        if (path.exit) edge(path.exit, endpoint(stopID, 'in'));
      } else if (path.exit) { edge(path.exit, merge.entry); hasContinuation = true; }
    };
    if (isSwitch) {
      branches.forEach((_, i) => branchBody(i, first.exit!, i === defaultIndex ? '' : metadata[i]?.title || `Case ${i + 1}`));
      if (defaultIndex < 0) { edge(first.exit!, merge.entry); hasContinuation = true; }
    } else {
      let current = first;
      const tested = branches.map((_, i) => i).filter(i => i !== defaultIndex);
      for (const [j, i] of tested.entries()) {
        if (j > 0) {
          const next = decision(`${n.id}-decision-${i}`, i);
          edge(current.alternative!, next.entry);
          current = next;
        }
        branchBody(i, current.exit!, metadata[i]?.label || 'Yes');
      }
      if (defaultIndex >= 0) branchBody(defaultIndex, current.alternative!, '');
      else { edge(current.alternative!, merge.entry); hasContinuation = true; }
    }
    if (hasContinuation && !isSwitch) {
      // A layout-only constraint gives ELK a preferred vertical spine through
      // the choice. It is deliberately absent from edgeEndpoints, so no fake
      // execution edge is emitted. Actual branches retain ELK's own routing.
      const start = records.get(n.id)!.node;
      start.ports!.push({ id: `${n.id}-spine`, width: 0, height: 0, layoutOptions: { 'elk.port.side': 'SOUTH' } });
      graph.edges!.push({
        id: `${n.id}-layout-spine`, sources: [`${n.id}-spine`], targets: [merge.entry.port],
        layoutOptions: { 'elk.layered.priority.straightness': '1000' },
      });
    }
    // No dangling merge when all alternatives return. In particular, the
    // caller must not draw a continuation that Go can never take.
    if (!hasContinuation) {
      parent.children = parent.children!.filter(child => child.id !== `${n.id}-merge`);
      records.delete(`${n.id}-merge`);
    }
    return { entry: first.entry, exit: hasContinuation ? merge.exit : undefined };
  }
  sequence(graph, page.body, 'workflow');
  const result = await elk.layout(graph);
  const nodes: WorkflowNode[] = [];
  const edges: WorkflowEdge[] = [];
  const origins = new Map<string, Point>([[graph.id, { x: 0, y: 0 }]]);
  function edgeOrigin(edge: ElkExtendedEdge, source: Endpoint, target: Endpoint): Point {
    // ELK leaves edges in their input array but reports section coordinates
    // relative to their lowest common ancestor, including edges inside scopes.
    const owner = (edge as ElkExtendedEdge & { container?: string }).container;
    if (owner && origins.has(owner)) return origins.get(owner)!;
    const ancestors = new Set<string>();
    let parent = records.get(source.node)?.parent;
    while (parent) { ancestors.add(parent); parent = records.get(parent)?.parent; }
    parent = records.get(target.node)?.parent;
    while (parent) {
      if (ancestors.has(parent)) return origins.get(parent)!;
      parent = records.get(parent)?.parent;
    }
    return { x: 0, y: 0 };
  }
  const offsetPoint = (p: Point, offset: Point): Point => ({ x: p.x + offset.x, y: p.y + offset.y });
  function collect(container: ElkNode, offset: Point): void {
    for (const child of container.children || []) {
      const record = records.get(child.id)!;
      const width = child.width || 8;
      const height = child.height || 8;
      const data = { ...record.data, width, height,
        ports: (child.ports || []).map(p => ({ id: p.id, x: p.x || 0, y: p.y || 0,
          side: (p.layoutOptions?.['elk.port.side'] || 'NORTH') as 'NORTH' | 'SOUTH' | 'EAST' | 'WEST' })),
      };
      nodes.push({
        id: child.id, type: record.data.kind === 'junction' ? 'junction' : 'workflow',
        position: { x: child.x || 0, y: child.y || 0 },
        parentId: record.parent, data, width, height, draggable: false, selectable: record.data.kind !== 'junction',
        style: `width: ${width}px; height: ${height}px;`,
      });
      const origin = { x: offset.x + (child.x || 0), y: offset.y + (child.y || 0) };
      origins.set(child.id, origin);
      collect(child, origin);
    }
    for (const edge of container.edges || []) {
      const endpoints = edgeEndpoints.get(edge.id);
      if (!endpoints) continue;
      const routeOrigin = edgeOrigin(edge, endpoints.source, endpoints.target);
      edges.push({ id: edge.id, type: 'routed', source: endpoints.source.node, target: endpoints.target.node,
        sourceHandle: endpoints.source.port, targetHandle: endpoints.target.port,
        selectable: false,
        data: {
          sections: (edge.sections || []).map(s => ({
            startPoint: offsetPoint(s.startPoint, routeOrigin), endPoint: offsetPoint(s.endPoint, routeOrigin),
            bendPoints: s.bendPoints?.map(p => offsetPoint(p, routeOrigin)),
          })),
          labels: (edge.labels || []).map(label => ({ text: label.text || '',
            x: (label.x || 0) + routeOrigin.x, y: (label.y || 0) + routeOrigin.y,
            width: label.width || 0, height: label.height || 0,
          })),
        },
      });
    }
  }
  collect(result, { x: 0, y: 0 });
  return { nodes, edges };
}
