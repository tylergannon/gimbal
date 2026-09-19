import type { PipState } from "./Pip.svelte";
import type { NodeOperation } from "./Node.svelte";
import type { RunSnapshot, ScopeRow, TurnRow } from "../observation/index.js";
import type {
  AgentCall,
  Graph,
  Group as GraphGroup,
  PromiseLoop,
  Supervisor,
} from "../workflow/types.js";

type Operation = Graph["body"][number];
type Sequence = Operation[];

type MeasureContext = {
  nodeWidth: number;
  gap: number;
};

type MeasuredNode = {
  kind: "node";
  operation: NodeOperation;
  width: number;
  height: number;
};

type MeasuredBranch = {
  name: string;
  body: MeasuredSequence;
  width: number;
  contentTop: number;
  height: number;
};

type MeasuredGroup = {
  kind: "group";
  operation: GraphGroup & { kind: "group" };
  branches: MeasuredBranch[];
  width: number;
  height: number;
};

type MeasuredLoop = {
  kind: "loop";
  operation: PromiseLoop & { kind: "promise_loop" };
  planner: MeasuredNode;
  body: MeasuredSequence;
  width: number;
  height: number;
  taskHeight: number;
};

type MeasuredScope = {
  kind: "scope";
  operation: Extract<Operation, { kind: "scope" | "iterate" }>;
  body: MeasuredSequence;
  width: number;
  height: number;
};

type MeasuredBlock = MeasuredNode | MeasuredGroup | MeasuredLoop | MeasuredScope;

type MeasuredSequence = {
  blocks: MeasuredBlock[];
  width: number;
  height: number;
  gap: number;
};

export type MapNodeLayout = {
  kind: "node";
  operation: NodeOperation;
  scopeKey: string;
  x: number;
  y: number;
  width: number;
  height: number;
  state: PipState;
  meta: string;
  selected: boolean;
};

export type MapSheetLayout = {
  kind: "scope" | "group" | "loop";
  scope: ScopeRow;
  instances: ScopeRow[];
  x: number;
  y: number;
  width: number;
  height: number;
  depth: number;
  contextTotal: number;
  selected: boolean;
  selectionPath: boolean;
};

export type GroupLayout = {
  group: GraphGroup & { kind: "group" };
  x: number;
  y: number;
  width: number;
  height: number;
  forkY: number;
  joinY: number;
  branches: Array<{ name: string; centerX: number; top: number; bottom: number }>;
};

export type LoopLayout = {
  loop: PromiseLoop & { kind: "promise_loop" };
  x: number;
  y: number;
  width: number;
  height: number;
  centerX: number;
  plannerRight: number;
  plannerCenterY: number;
  bodyRight: number;
  bodyExitY: number;
  returnX: number;
  returnBottomY: number;
};

export type WatcherLayout = {
  supervisor: Supervisor;
  state: PipState;
  x: number;
  y: number;
  width: number;
  height: number;
  watchedScope: string;
};

export type MapLayout = {
  width: number;
  height: number;
  centerX: number;
  start: { x: number; y: number };
  end: { x: number; y: number };
  connections: string[];
  watcherBrackets: string[];
  nodes: MapNodeLayout[];
  sheets: MapSheetLayout[];
  groups: GroupLayout[];
  loops: LoopLayout[];
  watchers: WatcherLayout[];
};

type Placement = {
  snapshot: RunSnapshot;
  nodes: MapNodeLayout[];
  sheets: MapSheetLayout[];
  groups: GroupLayout[];
  loops: LoopLayout[];
  watchers: WatcherLayout[];
  connections: string[];
  watcherBrackets: string[];
};

type PlacedBlock = { entryY: number; exitY: number };

const canvasWidth = 1040;
const centerX = 560;
const startY = 30;
const firstTop = 64;
const nodeWidth = 360;
const nodeGap = 20;
const groupInset = 8;
const branchGap = 8;
const groupChildTop = 36;
const groupBottom = 24;
const loopWidth = 600;
const loopPlannerTop = 28;
const loopTaskTop = 96;
const loopTaskWidth = 556;
const loopTaskBodyTop = 32;
const loopTaskBottom = 8;
const loopBottom = 32;
const watcherWidth = 248;
const watcherHeight = 36;
const watcherGap = 12;

function isNode(operation: Operation): operation is NodeOperation {
  return (
    operation.kind === "agent_call" ||
    operation.kind === "command" ||
    operation.kind === "interview"
  );
}

function measureNode(operation: NodeOperation, width: number): MeasuredNode {
  return {
    kind: "node",
    operation,
    width,
    height: operation.kind === "command" ? 36 : 44,
  };
}

function visibleOperations(operation: Operation): Sequence {
  if (operation.kind === "condition") {
    return operation.branches.flatMap((branch) =>
      branch.body.flatMap((child) => visibleOperations(child as Operation)),
    );
  }
  if (operation.kind === "repeat") {
    return operation.body.flatMap((child) => visibleOperations(child as Operation));
  }
  if (operation.kind === "session" || operation.kind === "set") return [];
  return [operation];
}

function firstVisibleOperation(operations: Sequence) {
  return operations.flatMap(visibleOperations).at(0);
}

function measureOperation(operation: Operation, context: MeasureContext): MeasuredBlock[] {
  if (isNode(operation)) return [measureNode(operation, context.nodeWidth)];

  if (operation.kind === "condition" || operation.kind === "repeat") {
    return measureSequence(
      operation.kind === "condition"
        ? operation.branches.flatMap((branch) => branch.body as Sequence)
        : (operation.body as Sequence),
      context,
    ).blocks;
  }

  if (operation.kind === "session" || operation.kind === "set") return [];

  if (operation.kind === "group") {
    const branches = operation.children.map((child) => {
      const first = firstVisibleOperation(child.body as Sequence);
      const contentTop = first?.kind === "interview" ? 32 : 24;
      const body = measureSequence(child.body as Sequence, { nodeWidth: 272, gap: context.gap });
      const width = Math.max(288, body.width + 16);
      const height = Math.max(64, contentTop + body.height + 12);
      return { name: child.name, body, width, contentTop, height };
    });
    const width =
      branches.reduce((sum, branch) => sum + branch.width, groupInset * 2) +
      Math.max(0, branches.length - 1) * branchGap;
    const tallest = Math.max(0, ...branches.map((branch) => branch.height));
    return [
      {
        kind: "group",
        operation,
        branches,
        width,
        height: groupChildTop + tallest + groupBottom,
      },
    ];
  }

  if (operation.kind === "promise_loop") {
    const planner: AgentCall & { kind: "agent_call" } = {
      kind: "agent_call",
      file: operation.file,
      line: operation.line,
      session: operation.planner,
      role: operation.planner,
      prompt: "",
      supervisors: operation.supervisors,
    };
    const body = measureSequence(operation.body as Sequence, {
      nodeWidth,
      gap: nodeGap,
    });
    const taskHeight = loopTaskBodyTop + body.height + loopTaskBottom;
    return [
      {
        kind: "loop",
        operation,
        planner: measureNode(planner, nodeWidth),
        body,
        width: loopWidth,
        taskHeight,
        height: loopTaskTop + taskHeight + loopBottom,
      },
    ];
  }

  const body = measureSequence(operation.body as Sequence, context);
  const width = Math.max(context.nodeWidth + 16, body.width + 16);
  return [
    {
      kind: "scope",
      operation,
      body,
      width,
      height: 32 + body.height + 8,
    },
  ];
}

function measureSequence(operations: Sequence, context: MeasureContext): MeasuredSequence {
  const blocks = operations.flatMap((operation) => measureOperation(operation, context));
  return {
    blocks,
    width: Math.max(0, ...blocks.map((block) => block.width)),
    height:
      blocks.reduce((sum, block) => sum + block.height, 0) +
      Math.max(0, blocks.length - 1) * context.gap,
    gap: context.gap,
  };
}

function parentKey(key: string) {
  const separator = key.lastIndexOf("/");
  return separator < 0 ? "" : key.slice(0, separator);
}

function childScopes(snapshot: RunSnapshot, parent: string, name: string) {
  return Object.values(snapshot.scopes)
    .filter((scope) => parentKey(scope.key) === parent && scope.name === name)
    .sort((left, right) => left.began - right.began);
}

function activeScope(snapshot: RunSnapshot, parent: string, name: string) {
  return childScopes(snapshot, parent, name).at(-1);
}

function turnFor(snapshot: RunSnapshot, scopeKey: string, sessionName: string) {
  const sessionIDs = new Set(
    Object.values(snapshot.sessions)
      .filter((session) => session.name === sessionName)
      .map((session) => session.id),
  );
  return Object.values(snapshot.turns)
    .filter((turn) => turn.scope === scopeKey && sessionIDs.has(turn.session))
    .sort((left, right) => left.started - right.started)
    .at(-1);
}

function turnState(turn?: TurnRow): PipState {
  if (!turn) return "not-yet";
  if (turn.error) return "failed";
  return turn.ended === 0 ? "running" : "ended";
}

function operationState(snapshot: RunSnapshot, scopeKey: string, operation: NodeOperation) {
  if (operation.kind === "command") {
    const command = Object.values(snapshot.commands ?? {}).find(
      (row) => row.scope === scopeKey && row.name === operation.name,
    );
    if (!command) return "not-yet" as const;
    if (command.error || command.exit_code !== 0) return "failed" as const;
    return command.ended === 0 ? ("running" as const) : ("ended" as const);
  }

  if (operation.kind === "interview") {
    const interview = Object.values(snapshot.interviews)
      .filter((row) => row.scope === scopeKey && row.name === operation.name)
      .sort((left, right) => left.asked - right.asked)
      .at(-1);
    if (!interview) return "not-yet" as const;
    return interview.status === "pending" ? ("waiting" as const) : ("ended" as const);
  }

  return turnState(turnFor(snapshot, scopeKey, operation.session));
}

function operationMeta(snapshot: RunSnapshot, scopeKey: string, operation: NodeOperation) {
  if (operation.kind === "command") {
    const command = Object.values(snapshot.commands ?? {}).find(
      (row) => row.scope === scopeKey && row.name === operation.name,
    );
    return command ? `exit ${command.exit_code}` : "not started";
  }

  if (operation.kind === "interview") {
    const interviews = Object.values(snapshot.interviews)
      .filter((row) => row.scope === scopeKey && row.name === operation.name)
      .sort((left, right) => left.asked - right.asked);
    const current = interviews.at(-1);
    if (!current) return "not started";
    return `question ${interviews.length}`;
  }

  const turn = turnFor(snapshot, scopeKey, operation.session);
  if (!turn) return "not started";
  if (operation.prompt === "") return "planner";
  const sessionIDs = new Set(
    Object.values(snapshot.sessions)
      .filter((session) => session.name === operation.session)
      .map((session) => session.id),
  );
  const sessionTurns = Object.values(snapshot.turns).filter((row) => sessionIDs.has(row.session));
  if (sessionTurns.length === 1) return "";
  const match = /turn\.(\d+)$/.exec(turn.id);
  return match ? `turn ${match[1]}` : "turn";
}

function collectSetKeys(operations: Sequence): Set<string> {
  const keys = new Set<string>();
  for (const operation of operations) {
    if (operation.kind === "set") keys.add(operation.key);
    if (operation.kind === "condition") {
      for (const branch of operation.branches) {
        for (const key of collectSetKeys(branch.body as Sequence)) keys.add(key);
      }
    } else if (
      operation.kind === "repeat" ||
      operation.kind === "scope" ||
      operation.kind === "iterate"
    ) {
      for (const key of collectSetKeys(operation.body as Sequence)) keys.add(key);
    }
  }
  return keys;
}

function addSheet(
  placement: Placement,
  kind: MapSheetLayout["kind"],
  scope: ScopeRow,
  instances: ScopeRow[],
  x: number,
  y: number,
  width: number,
  height: number,
  depth: number,
  contextTotal = 0,
) {
  placement.sheets.push({
    kind,
    scope,
    instances,
    x,
    y,
    width,
    height,
    depth,
    contextTotal,
    selected: false,
    selectionPath: false,
  });
}

function placeNode(
  placement: Placement,
  node: MeasuredNode,
  scopeKey: string,
  x: number,
  y: number,
) {
  placement.nodes.push({
    kind: "node",
    operation: node.operation,
    scopeKey,
    x,
    y,
    width: node.width,
    height: node.height,
    state: operationState(placement.snapshot, scopeKey, node.operation),
    meta: operationMeta(placement.snapshot, scopeKey, node.operation),
    selected: false,
  });
  return { entryY: y, exitY: y + node.height };
}

function placeSequence(
  placement: Placement,
  sequence: MeasuredSequence,
  scopeKey: string,
  center: number,
  top: number,
  depth: number,
): PlacedBlock | undefined {
  let cursor = top;
  let first: PlacedBlock | undefined;
  let previous: PlacedBlock | undefined;

  for (const block of sequence.blocks) {
    const placed = placeBlock(placement, block, scopeKey, center, cursor, depth);
    if (!first) first = placed;
    if (previous) placement.connections.push(`M${center},${previous.exitY} V${placed.entryY}`);
    previous = placed;
    cursor += block.height + sequence.gap;
  }

  if (!first || !previous) return undefined;
  return { entryY: first.entryY, exitY: previous.exitY };
}

function placeGroup(
  placement: Placement,
  group: MeasuredGroup,
  parentScope: string,
  center: number,
  top: number,
  depth: number,
): PlacedBlock {
  const x = center - group.width / 2;
  const instances = childScopes(placement.snapshot, parentScope, group.operation.name);
  const scope = instances.at(-1);
  const forkY = 20;
  let branchX = groupInset;
  const branches: GroupLayout["branches"] = [];

  if (scope)
    addSheet(placement, "group", scope, instances, x, top, group.width, group.height, depth);

  for (const branch of group.branches) {
    const branchInstances = scope ? childScopes(placement.snapshot, scope.key, branch.name) : [];
    const branchScope = branchInstances.at(-1);
    const branchTop = groupChildTop;
    const branchCenter = branchX + branch.width / 2;
    branches.push({
      name: branch.name,
      centerX: branchCenter,
      top: branchTop,
      bottom: branchTop + branch.height,
    });

    if (branchScope) {
      addSheet(
        placement,
        "scope",
        branchScope,
        branchInstances,
        x + branchX,
        top + branchTop,
        branch.width,
        branch.height,
        depth + 1,
      );
      placeSequence(
        placement,
        branch.body,
        branchScope.key,
        x + branchCenter,
        top + branchTop + branch.contentTop,
        depth + 1,
      );
    }
    branchX += branch.width + branchGap;
  }

  const joinY = group.height - 12;
  placement.groups.push({
    group: group.operation,
    x,
    y: top,
    width: group.width,
    height: group.height,
    forkY,
    joinY,
    branches,
  });
  return { entryY: top + forkY, exitY: top + joinY };
}

function watcherState(snapshot: RunSnapshot, scopeKey: string, supervisor: Supervisor) {
  return turnState(turnFor(snapshot, scopeKey, supervisor.session));
}

function addWatchers(
  placement: Placement,
  supervisors: Supervisor[],
  watchedScope: string,
  watchedNode: { x: number; y: number },
) {
  if (supervisors.length === 0) return;
  const left = 16;
  const firstY = watchedNode.y - 20;
  supervisors.forEach((supervisor, index) => {
    placement.watchers.push({
      supervisor,
      state: watcherState(placement.snapshot, watchedScope, supervisor),
      x: left,
      y: firstY + index * (watcherHeight + watcherGap),
      width: watcherWidth,
      height: watcherHeight,
      watchedScope,
    });
  });
  const bracketX = left + watcherWidth;
  const targetX = watchedNode.x - 2;
  const topY = firstY + watcherHeight / 2;
  const bottomY =
    firstY + (supervisors.length - 1) * (watcherHeight + watcherGap) + watcherHeight / 2;
  const middleY = (topY + bottomY) / 2;
  placement.watcherBrackets.push(
    `M${bracketX},${topY} H${bracketX + 8} V${bottomY} H${bracketX} M${bracketX + 8},${middleY} H${targetX}`,
  );
}

function placeLoop(
  placement: Placement,
  loop: MeasuredLoop,
  parentScope: string,
  center: number,
  top: number,
  depth: number,
): PlacedBlock {
  const x = center - loop.width / 2;
  const instances = childScopes(placement.snapshot, parentScope, loop.operation.name);
  const scope = instances.at(-1);
  const loopScopeKey = scope?.key ?? parentScope;
  if (scope) addSheet(placement, "loop", scope, instances, x, top, loop.width, loop.height, depth);

  const plannerX = center - loop.planner.width / 2;
  const plannerY = top + loopPlannerTop;
  const planner = placeNode(placement, loop.planner, loopScopeKey, plannerX, plannerY);

  const taskInstances = scope ? childScopes(placement.snapshot, scope.key, "task") : [];
  const taskScope = taskInstances.at(-1);
  const taskX = x + groupInset;
  const taskY = top + loopTaskTop;
  if (taskScope) {
    addSheet(
      placement,
      "scope",
      taskScope,
      taskInstances,
      taskX,
      taskY,
      loopTaskWidth,
      loop.taskHeight,
      depth + 1,
      collectSetKeys(loop.operation.body as Sequence).size + 1,
    );
  }

  const bodyScopeKey = taskScope?.key ?? loopScopeKey;
  const body = placeSequence(
    placement,
    loop.body,
    bodyScopeKey,
    center,
    taskY + loopTaskBodyTop,
    depth + 1,
  );
  if (body) {
    placement.connections.push(`M${center},${planner.exitY} V${body.entryY}`);
  }

  const bodyNodes = placement.nodes.filter(
    (node) => node.scopeKey === bodyScopeKey && node.y >= taskY && node.y < taskY + loop.taskHeight,
  );
  for (const node of bodyNodes) {
    if (node.operation.kind === "agent_call") {
      addWatchers(placement, node.operation.supervisors, bodyScopeKey, node);
    }
  }

  const bodyExitY = (body?.exitY ?? planner.exitY) - top;
  placement.loops.push({
    loop: loop.operation,
    x,
    y: top,
    width: loop.width,
    height: loop.height,
    centerX: center - x,
    plannerRight: plannerX + loop.planner.width + 5 - x,
    plannerCenterY: plannerY + loop.planner.height / 2 - top,
    bodyRight: taskX + loopTaskWidth - x,
    bodyExitY,
    returnX: loop.width - 20,
    returnBottomY: loop.height - 14,
  });
  return { entryY: planner.entryY, exitY: top + loop.height };
}

function placeScope(
  placement: Placement,
  measured: MeasuredScope,
  parentScope: string,
  center: number,
  top: number,
  depth: number,
): PlacedBlock {
  const x = center - measured.width / 2;
  const instances = childScopes(placement.snapshot, parentScope, measured.operation.name);
  const scope = instances.at(-1);
  if (scope) {
    addSheet(
      placement,
      "scope",
      scope,
      instances,
      x,
      top,
      measured.width,
      measured.height,
      depth,
      collectSetKeys(measured.operation.body as Sequence).size,
    );
  }
  const body = placeSequence(
    placement,
    measured.body,
    scope?.key ?? parentScope,
    center,
    top + 32,
    depth + 1,
  );
  return body ?? { entryY: top, exitY: top + measured.height };
}

function placeBlock(
  placement: Placement,
  block: MeasuredBlock,
  scopeKey: string,
  center: number,
  top: number,
  depth: number,
) {
  if (block.kind === "node") {
    return placeNode(placement, block, scopeKey, center - block.width / 2, top);
  }
  if (block.kind === "group") return placeGroup(placement, block, scopeKey, center, top, depth);
  if (block.kind === "loop") return placeLoop(placement, block, scopeKey, center, top, depth);
  return placeScope(placement, block, scopeKey, center, top, depth);
}

function isAncestorScope(ancestor: string, descendant: string) {
  return ancestor === descendant || (ancestor !== "" && descendant.startsWith(`${ancestor}/`));
}

function applyAutomaticSelection(placement: Placement) {
  const selected = placement.nodes.find(
    (node) => node.state === "running" || node.state === "waiting",
  );
  if (!selected) return;
  selected.selected = true;
  const matchingSheets = placement.sheets.filter((sheet) =>
    isAncestorScope(sheet.scope.key, selected.scopeKey),
  );
  for (const sheet of matchingSheets) sheet.selectionPath = true;
  const deepest = matchingSheets.sort(
    (left, right) => right.scope.key.split("/").length - left.scope.key.split("/").length,
  )[0];
  if (deepest) deepest.selected = true;
}

export function buildMapLayout(graph: Graph, snapshot: RunSnapshot): MapLayout {
  const measured = measureSequence(graph.body as Sequence, { nodeWidth, gap: nodeGap });
  const placement: Placement = {
    snapshot,
    nodes: [],
    sheets: [],
    groups: [],
    loops: [],
    watchers: [],
    connections: [],
    watcherBrackets: [],
  };

  let cursor = firstTop;
  let first: PlacedBlock | undefined;
  let previous: PlacedBlock | undefined;
  for (let index = 0; index < measured.blocks.length; index++) {
    const block = measured.blocks[index];
    const placed = placeBlock(placement, block, "", centerX, cursor, 0);
    if (!first) first = placed;
    if (previous) placement.connections.push(`M${centerX},${previous.exitY} V${placed.entryY}`);
    previous = placed;
    const next = measured.blocks[index + 1];
    cursor += block.height + (next?.kind === "loop" ? 40 : 36);
  }

  const firstEntry = first?.entryY ?? firstTop;
  const lastExit = previous?.exitY ?? firstTop;
  const endY = lastExit + 28;
  placement.connections.unshift(`M${centerX},${startY + 4} V${firstEntry}`);
  placement.connections.push(`M${centerX},${lastExit} V${endY - 4}`);
  applyAutomaticSelection(placement);

  return {
    width: canvasWidth,
    height: Math.max(360, endY + 24),
    centerX,
    start: { x: centerX, y: startY },
    end: { x: centerX, y: endY },
    connections: placement.connections,
    watcherBrackets: placement.watcherBrackets,
    nodes: placement.nodes,
    sheets: placement.sheets,
    groups: placement.groups,
    loops: placement.loops,
    watchers: placement.watchers,
  };
}
