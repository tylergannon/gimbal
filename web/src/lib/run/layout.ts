import type { PipState } from "./Pip.svelte";
import type { NodeOperation } from "./Node.svelte";
import type {
  CommandRow,
  InterviewRow,
  RunSnapshot,
  ScopeRow,
  TurnRow,
} from "../observation/index.svelte.js";
import type {
  AgentCall,
  Graph,
  Group as GraphGroup,
  PromiseLoop,
  Service,
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
  services: Service[];
  serviceTop: number;
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
  serviceHeight: number;
  bodyTop: number;
  width: number;
  height: number;
  taskHeight: number;
};

type MeasuredScope = {
  kind: "scope";
  operation: Extract<Operation, { kind: "scope" | "iterate" }>;
  body: MeasuredSequence;
  serviceHeight: number;
  contentTop: number;
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
  selectionKey: string;
  runtime?: TurnRow | CommandRow | InterviewRow;
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
  serviceCount: number;
  parentScopeKey: string;
  instanceGroupKey: string;
  selectionKey: string;
  folded: boolean;
  elapsed: string;
  steps: Array<{ operation: NodeOperation; state: PipState }>;
  selected: boolean;
  selectionPath: boolean;
};

export type MapServicesLayout = {
  scopeKey: string;
  items: Array<{ service: Service; selectionKey: string; selected: boolean }>;
  x: number;
  y: number;
  width: number;
  height: number;
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
  selectionKey: string;
  turn?: TurnRow;
  selected: boolean;
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
  services: MapServicesLayout[];
  sheets: MapSheetLayout[];
  groups: GroupLayout[];
  loops: LoopLayout[];
  watchers: WatcherLayout[];
  selectedKey?: string;
};

export type MapLayoutOptions = {
  selectedInstances?: Readonly<Record<string, string>>;
  foldedScopes?: readonly string[];
  selectedKey?: string;
  now?: number;
};

type Placement = {
  snapshot: RunSnapshot;
  options: MapLayoutOptions;
  foldedScopes: Set<string>;
  nodes: MapNodeLayout[];
  services: MapServicesLayout[];
  sheets: MapSheetLayout[];
  groups: GroupLayout[];
  loops: LoopLayout[];
  watchers: WatcherLayout[];
  connections: string[];
  watcherBrackets: string[];
};

type PlacedBlock = { entryY: number; exitY: number; height: number };

const canvasWidth = 1040;
const centerX = 560;
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
const foldedHeight = 64;
const servicePanelGap = 12;

function servicePanelHeight(services: readonly Service[]) {
  return services.length === 0 ? 0 : 32 + services.length * 30;
}

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
      const serviceTop = 24;
      const serviceHeight = servicePanelHeight(child.services);
      const contentTop = serviceHeight
        ? serviceTop + serviceHeight + servicePanelGap
        : first?.kind === "interview"
          ? 32
          : 24;
      const body = measureSequence(child.body as Sequence, { nodeWidth: 272, gap: context.gap });
      const width = Math.max(288, body.width + 16);
      const height = Math.max(64, contentTop + body.height + 12);
      return {
        name: child.name,
        services: child.services,
        serviceTop,
        body,
        width,
        contentTop,
        height,
      };
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
    const serviceHeight = servicePanelHeight(operation.services);
    const bodyTop = loopTaskBodyTop + serviceHeight + (serviceHeight ? servicePanelGap : 0);
    const taskHeight = bodyTop + body.height + loopTaskBottom;
    return [
      {
        kind: "loop",
        operation,
        planner: measureNode(planner, nodeWidth),
        body,
        serviceHeight,
        bodyTop,
        width: loopWidth,
        taskHeight,
        height: loopTaskTop + taskHeight + loopBottom,
      },
    ];
  }

  const body = measureSequence(operation.body as Sequence, context);
  const serviceHeight = servicePanelHeight(operation.services);
  const contentTop = serviceHeight ? 24 + serviceHeight + servicePanelGap : 32;
  const width = Math.max(context.nodeWidth + 16, body.width + 16);
  return [
    {
      kind: "scope",
      operation,
      body,
      serviceHeight,
      contentTop,
      width,
      height: contentTop + body.height + 8,
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

function declaredScopeName(scope: ScopeRow) {
  const runtimeName = scope.key.split("/").at(-1) ?? scope.name;
  return runtimeName.replace(/\.\d+$/, "");
}

function childScopes(snapshot: RunSnapshot, parent: string, name: string) {
  return Object.values(snapshot.scopes)
    .filter((scope) => parentKey(scope.key) === parent && declaredScopeName(scope) === name)
    .sort((left, right) => left.began - right.began);
}

function instanceGroupKey(parent: string, name: string) {
  return `${parent}/${name}`;
}

function selectedScope(
  placement: Placement,
  parent: string,
  name: string,
  instances = childScopes(placement.snapshot, parent, name),
) {
  const selected = placement.options.selectedInstances?.[instanceGroupKey(parent, name)];
  return instances.find((scope) => scope.key === selected) ?? instances.at(-1);
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

function commandFor(snapshot: RunSnapshot, scopeKey: string, name: string) {
  return Object.values(snapshot.commands ?? {})
    .filter((row) => row.scope === scopeKey && row.name === name)
    .sort((left, right) => left.started - right.started)
    .at(-1);
}

function interviewFor(snapshot: RunSnapshot, scopeKey: string, name: string) {
  return Object.values(snapshot.interviews)
    .filter((row) => row.scope === scopeKey && row.name === name)
    .sort((left, right) => left.asked - right.asked)
    .at(-1);
}

function runtimeFor(snapshot: RunSnapshot, scopeKey: string, operation: NodeOperation) {
  if (operation.kind === "command") return commandFor(snapshot, scopeKey, operation.name);
  if (operation.kind === "interview") return interviewFor(snapshot, scopeKey, operation.name);
  return turnFor(snapshot, scopeKey, operation.session);
}

function turnState(turn?: TurnRow): PipState {
  if (!turn) return "not-yet";
  if (turn.interrupted) return "ended";
  if (turn.error) return "failed";
  return turn.ended === 0 ? "running" : "ended";
}

function operationState(snapshot: RunSnapshot, scopeKey: string, operation: NodeOperation) {
  if (operation.kind === "command") {
    const command = commandFor(snapshot, scopeKey, operation.name);
    if (!command) return "not-yet" as const;
    if (command.interrupted) return "ended" as const;
    if (command.error || command.exit_code !== 0) return "failed" as const;
    return command.ended === 0 ? ("running" as const) : ("ended" as const);
  }

  if (operation.kind === "interview") {
    const interview = interviewFor(snapshot, scopeKey, operation.name);
    if (!interview) return "not-yet" as const;
    return interview.status === "pending" ? ("waiting" as const) : ("ended" as const);
  }

  return turnState(turnFor(snapshot, scopeKey, operation.session));
}

function operationMeta(snapshot: RunSnapshot, scopeKey: string, operation: NodeOperation) {
  if (operation.kind === "command") {
    const command = commandFor(snapshot, scopeKey, operation.name);
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

export function nodeSelectionKey(scopeKey: string, operation: NodeOperation) {
  const name = operation.kind === "agent_call" ? operation.session : operation.name;
  return `node:${scopeKey}:${operation.kind}:${name}:${operation.file}:${operation.line}`;
}

export function serviceSelectionKey(scopeKey: string, service: Service) {
  return `service:${scopeKey}:${service.name}:${service.file}:${service.line}`;
}

export function sheetSelectionKey(scopeKey: string) {
  return `sheet:${scopeKey}`;
}

export function watcherSelectionKey(scopeKey: string, supervisor: Supervisor) {
  return `watcher:${scopeKey}:${supervisor.session}:${supervisor.file}:${supervisor.line}`;
}

function observedAt(snapshot: RunSnapshot) {
  return Math.max(
    snapshot.run.started,
    ...Object.values(snapshot.scopes).map((scope) => scope.ended || scope.began),
    ...Object.values(snapshot.turns).map((turn) => turn.ended || turn.started + turn.duration),
    ...Object.values(snapshot.commands ?? {}).map(
      (command) => command.ended || command.started + command.duration,
    ),
  );
}

function elapsedFor(snapshot: RunSnapshot, scope: ScopeRow, now?: number) {
  const recordedAt = snapshot.run.ended || observedAt(snapshot);
  const endpoint =
    scope.ended || (snapshot.run.status === "running" ? (now ?? observedAt(snapshot)) : recordedAt);
  const milliseconds = Math.max(0, endpoint - scope.began);
  const minutes = Math.floor(milliseconds / 60_000);
  const seconds = Math.floor(milliseconds / 1_000) % 60;
  return `${minutes}m ${seconds.toString().padStart(2, "0")}s`;
}

function flattenedNodes(operations: Sequence): NodeOperation[] {
  return operations.flatMap((operation) => {
    if (isNode(operation)) return [operation];
    if (operation.kind === "condition") {
      return operation.branches.flatMap((branch) => flattenedNodes(branch.body as Sequence));
    }
    if (operation.kind === "repeat") return flattenedNodes(operation.body as Sequence);
    return [];
  });
}

function foldedSteps(snapshot: RunSnapshot, scopeKey: string, operations: Sequence) {
  return flattenedNodes(operations).map((operation) => ({
    operation,
    state: operationState(snapshot, scopeKey, operation),
  }));
}

function groupFoldedSteps(
  placement: Placement,
  group: GraphGroup & { kind: "group" },
  groupScope: ScopeRow,
) {
  return group.children.flatMap((child) => {
    const instances = childScopes(placement.snapshot, groupScope.key, child.name);
    const scope = selectedScope(placement, groupScope.key, child.name, instances);
    return foldedSteps(placement.snapshot, scope?.key ?? groupScope.key, child.body as Sequence);
  });
}

function loopFoldedSteps(placement: Placement, loop: MeasuredLoop, loopScope: ScopeRow) {
  const taskInstances = childScopes(placement.snapshot, loopScope.key, "task");
  const taskScope = selectedScope(placement, loopScope.key, "task", taskInstances);
  return [
    ...foldedSteps(placement.snapshot, loopScope.key, [loop.planner.operation]),
    ...foldedSteps(
      placement.snapshot,
      taskScope?.key ?? loopScope.key,
      loop.operation.body as Sequence,
    ),
  ];
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
  parentScopeKey: string,
  x: number,
  y: number,
  width: number,
  height: number,
  depth: number,
  contextTotal = 0,
  serviceCount = 0,
  operations: Sequence = [],
  steps?: Array<{ operation: NodeOperation; state: PipState }>,
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
    serviceCount,
    parentScopeKey,
    instanceGroupKey: instanceGroupKey(parentScopeKey, declaredScopeName(scope)),
    selectionKey: sheetSelectionKey(scope.key),
    folded: placement.foldedScopes.has(scope.key),
    elapsed: elapsedFor(placement.snapshot, scope, placement.options.now),
    steps: steps ?? foldedSteps(placement.snapshot, scope.key, operations),
    selected: false,
    selectionPath: false,
  });
}

function addServices(
  placement: Placement,
  services: Service[],
  scopeKey: string,
  center: number,
  top: number,
  width: number,
) {
  const height = servicePanelHeight(services);
  if (height === 0) return;
  placement.services.push({
    scopeKey,
    items: services.map((service) => ({
      service,
      selectionKey: serviceSelectionKey(scopeKey, service),
      selected: false,
    })),
    x: center - width / 2,
    y: top,
    width,
    height,
  });
}

function addServiceBypass(
  placement: Placement,
  center: number,
  panelWidth: number,
  fromY: number,
  toY: number,
) {
  if (toY <= fromY) return;
  const side = center + panelWidth / 2 + 4;
  placement.connections.push(`M${center},${fromY} H${side} V${toY} H${center}`);
}

function placeNode(
  placement: Placement,
  node: MeasuredNode,
  scopeKey: string,
  x: number,
  y: number,
) {
  const runtime = runtimeFor(placement.snapshot, scopeKey, node.operation);
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
    selectionKey: nodeSelectionKey(scopeKey, node.operation),
    runtime,
    selected: false,
  });
  return { entryY: y, exitY: y + node.height, height: node.height };
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
    cursor += placed.height + sequence.gap;
  }

  if (!first || !previous) return undefined;
  return { entryY: first.entryY, exitY: previous.exitY, height: previous.exitY - top };
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
  const scope = selectedScope(placement, parentScope, group.operation.name, instances);
  if (scope && placement.foldedScopes.has(scope.key)) {
    addSheet(
      placement,
      "group",
      scope,
      instances,
      parentScope,
      x,
      top,
      group.width,
      foldedHeight,
      depth,
      0,
      0,
      group.operation.children.flatMap((child) => child.body as Sequence),
      groupFoldedSteps(placement, group.operation, scope),
    );
    return { entryY: top, exitY: top + foldedHeight, height: foldedHeight };
  }

  const forkY = 20;
  let branchX = groupInset;
  const branches: GroupLayout["branches"] = [];
  let tallest = 0;

  for (const branch of group.branches) {
    const branchInstances = scope ? childScopes(placement.snapshot, scope.key, branch.name) : [];
    const branchScope = scope
      ? selectedScope(placement, scope.key, branch.name, branchInstances)
      : undefined;
    const branchTop = groupChildTop;
    const branchCenter = branchX + branch.width / 2;
    const branchFolded = branchScope ? placement.foldedScopes.has(branchScope.key) : false;
    let branchHeight = branchFolded ? foldedHeight : branch.height;

    if (branchScope && !branchFolded) {
      addServices(
        placement,
        branch.services,
        branchScope.key,
        x + branchCenter,
        top + branchTop + branch.serviceTop,
        branch.width - 16,
      );
      const body = placeSequence(
        placement,
        branch.body,
        branchScope.key,
        x + branchCenter,
        top + branchTop + branch.contentTop,
        depth + 1,
      );
      branchHeight = Math.max(foldedHeight, branch.contentTop + (body?.height ?? 0) + 12);
    }

    branches.push({
      name: branch.name,
      centerX: branchCenter,
      top: branchTop,
      bottom: branchTop + branchHeight,
    });

    if (branchScope) {
      addSheet(
        placement,
        "scope",
        branchScope,
        branchInstances,
        scope?.key ?? parentScope,
        x + branchX,
        top + branchTop,
        branch.width,
        branchHeight,
        depth + 1,
        0,
        branch.services.length,
        (group.operation.children.find((child) => child.name === branch.name)?.body ??
          []) as Sequence,
      );
    }
    tallest = Math.max(tallest, branchHeight);
    branchX += branch.width + branchGap;
  }

  const height = groupChildTop + tallest + groupBottom;
  if (scope) {
    addSheet(
      placement,
      "group",
      scope,
      instances,
      parentScope,
      x,
      top,
      group.width,
      height,
      depth,
      0,
      0,
      group.operation.children.flatMap((child) => child.body as Sequence),
      groupFoldedSteps(placement, group.operation, scope),
    );
  }

  const joinY = height - 12;
  placement.groups.push({
    group: group.operation,
    x,
    y: top,
    width: group.width,
    height,
    forkY,
    joinY,
    branches,
  });
  return { entryY: top + forkY, exitY: top + joinY, height };
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
    const turn = turnFor(placement.snapshot, watchedScope, supervisor.session);
    placement.watchers.push({
      supervisor,
      state: watcherState(placement.snapshot, watchedScope, supervisor),
      x: left,
      y: firstY + index * (watcherHeight + watcherGap),
      width: watcherWidth,
      height: watcherHeight,
      watchedScope,
      selectionKey: watcherSelectionKey(watchedScope, supervisor),
      turn,
      selected: false,
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
  const scope = selectedScope(placement, parentScope, loop.operation.name, instances);
  const loopScopeKey = scope?.key ?? parentScope;
  if (scope && placement.foldedScopes.has(scope.key)) {
    addSheet(
      placement,
      "loop",
      scope,
      instances,
      parentScope,
      x,
      top,
      loop.width,
      foldedHeight,
      depth,
      0,
      0,
      loop.operation.body as Sequence,
      loopFoldedSteps(placement, loop, scope),
    );
    return { entryY: top, exitY: top + foldedHeight, height: foldedHeight };
  }

  const plannerX = center - loop.planner.width / 2;
  const plannerY = top + loopPlannerTop;
  const planner = placeNode(placement, loop.planner, loopScopeKey, plannerX, plannerY);

  const taskInstances = scope ? childScopes(placement.snapshot, scope.key, "task") : [];
  const taskScope = scope ? selectedScope(placement, scope.key, "task", taskInstances) : undefined;
  const taskX = x + groupInset;
  const taskY = top + loopTaskTop;
  const taskFolded = taskScope ? placement.foldedScopes.has(taskScope.key) : false;

  const bodyScopeKey = taskScope?.key ?? loopScopeKey;
  if (!taskFolded) {
    addServices(
      placement,
      loop.operation.services,
      bodyScopeKey,
      center,
      taskY + loopTaskBodyTop,
      nodeWidth,
    );
  }
  const body = taskFolded
    ? undefined
    : placeSequence(placement, loop.body, bodyScopeKey, center, taskY + loop.bodyTop, depth + 1);
  const taskHeight = taskFolded
    ? foldedHeight
    : loop.bodyTop + (body?.height ?? 0) + loopTaskBottom;
  const height = loopTaskTop + taskHeight + loopBottom;

  if (taskScope) {
    addSheet(
      placement,
      "scope",
      taskScope,
      taskInstances,
      scope?.key ?? parentScope,
      taskX,
      taskY,
      loopTaskWidth,
      taskHeight,
      depth + 1,
      collectSetKeys(loop.operation.body as Sequence).size + 1,
      loop.operation.services.length,
      loop.operation.body as Sequence,
    );
  }
  if (loop.serviceHeight > 0) {
    placement.connections.push(`M${center},${planner.exitY} V${taskY}`);
    addServiceBypass(placement, center, nodeWidth, taskY, body?.entryY ?? taskY + taskHeight);
  } else {
    placement.connections.push(`M${center},${planner.exitY} V${body?.entryY ?? taskY}`);
  }

  const bodyNodes = placement.nodes.filter(
    (node) => node.scopeKey === bodyScopeKey && node.y >= taskY && node.y < taskY + taskHeight,
  );
  for (const node of bodyNodes) {
    if (node.operation.kind === "agent_call") {
      addWatchers(placement, node.operation.supervisors, bodyScopeKey, node);
    }
  }

  const bodyExitY = (body?.exitY ?? taskY + taskHeight) - top;
  placement.loops.push({
    loop: loop.operation,
    x,
    y: top,
    width: loop.width,
    height,
    centerX: center - x,
    plannerRight: plannerX + loop.planner.width + 5 - x,
    plannerCenterY: plannerY + loop.planner.height / 2 - top,
    bodyRight: taskX + loopTaskWidth - x,
    bodyExitY,
    returnX: loop.width - 20,
    returnBottomY: height - 14,
  });
  if (scope) {
    addSheet(
      placement,
      "loop",
      scope,
      instances,
      parentScope,
      x,
      top,
      loop.width,
      height,
      depth,
      0,
      0,
      loop.operation.body as Sequence,
      loopFoldedSteps(placement, loop, scope),
    );
  }
  return { entryY: planner.entryY, exitY: top + height, height };
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
  const scope = selectedScope(placement, parentScope, measured.operation.name, instances);
  if (scope && placement.foldedScopes.has(scope.key)) {
    addSheet(
      placement,
      "scope",
      scope,
      instances,
      parentScope,
      x,
      top,
      measured.width,
      foldedHeight,
      depth,
      collectSetKeys(measured.operation.body as Sequence).size,
      measured.operation.services.length,
      measured.operation.body as Sequence,
    );
    return { entryY: top, exitY: top + foldedHeight, height: foldedHeight };
  }
  const scopeKey = scope?.key ?? parentScope;
  addServices(
    placement,
    measured.operation.services,
    scopeKey,
    center,
    top + 24,
    measured.width - 16,
  );
  const body = placeSequence(
    placement,
    measured.body,
    scopeKey,
    center,
    top + measured.contentTop,
    depth + 1,
  );
  const height = Math.max(foldedHeight, measured.contentTop + (body?.height ?? 0) + 8);
  if (measured.serviceHeight > 0) {
    addServiceBypass(placement, center, measured.width - 16, top, body?.entryY ?? top + height);
    if (body) placement.connections.push(`M${center},${body.exitY} V${top + height}`);
  }
  if (scope) {
    addSheet(
      placement,
      "scope",
      scope,
      instances,
      parentScope,
      x,
      top,
      measured.width,
      height,
      depth,
      collectSetKeys(measured.operation.body as Sequence).size,
      measured.operation.services.length,
      measured.operation.body as Sequence,
    );
  }
  return body
    ? { entryY: measured.serviceHeight > 0 ? top : body.entryY, exitY: top + height, height }
    : { entryY: top, exitY: top + height, height };
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

function applySelection(placement: Placement) {
  let selectedScope = "";
  let selectedKey = placement.options.selectedKey;
  const selectedNode = placement.nodes.find((node) => node.selectionKey === selectedKey);
  const selectedService = placement.services
    .flatMap((group) => group.items.map((item) => ({ group, item })))
    .find(({ item }) => item.selectionKey === selectedKey);
  const selectedWatcher = placement.watchers.find(
    (watcher) => watcher.selectionKey === selectedKey,
  );
  const selectedSheet = placement.sheets.find((sheet) => sheet.selectionKey === selectedKey);

  if (selectedNode) {
    selectedNode.selected = true;
    selectedScope = selectedNode.scopeKey;
  } else if (selectedService) {
    selectedService.item.selected = true;
    selectedScope = selectedService.group.scopeKey;
  } else if (selectedWatcher) {
    selectedWatcher.selected = true;
    selectedScope = selectedWatcher.watchedScope;
  } else if (selectedSheet) {
    selectedSheet.selected = true;
    selectedScope = selectedSheet.scope.key;
  } else return undefined;

  const matchingSheets = placement.sheets.filter((sheet) =>
    isAncestorScope(sheet.scope.key, selectedScope),
  );
  for (const sheet of matchingSheets) sheet.selectionPath = true;
  if (!selectedSheet) {
    const deepest = matchingSheets.sort(
      (left, right) => right.scope.key.split("/").length - left.scope.key.split("/").length,
    )[0];
    if (deepest) deepest.selected = true;
  }
  return selectedKey;
}

export function buildMapLayout(
  graph: Graph,
  snapshot: RunSnapshot,
  options: MapLayoutOptions = {},
): MapLayout {
  const measured = measureSequence(graph.body as Sequence, { nodeWidth, gap: nodeGap });
  const placement: Placement = {
    snapshot,
    options,
    foldedScopes: new Set(options.foldedScopes),
    nodes: [],
    services: [],
    sheets: [],
    groups: [],
    loops: [],
    watchers: [],
    connections: [],
    watcherBrackets: [],
  };

  const rootServiceTop = graph.services.length > 0 ? 20 : 0;
  const rootServiceHeight = servicePanelHeight(graph.services);
  addServices(placement, graph.services, "", centerX, rootServiceTop, nodeWidth);
  const startY = rootServiceHeight > 0 ? rootServiceTop + rootServiceHeight + 24 : 30;
  const firstTop = startY + 34;
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
    cursor += placed.height + (next?.kind === "loop" ? 40 : 36);
  }

  const firstEntry = first?.entryY ?? firstTop;
  const lastExit = previous?.exitY ?? firstTop;
  const endY = lastExit + 28;
  placement.connections.unshift(`M${centerX},${startY + 4} V${firstEntry}`);
  placement.connections.push(`M${centerX},${lastExit} V${endY - 4}`);
  const selectedKey = applySelection(placement);

  return {
    width: canvasWidth,
    height: Math.max(360, endY + 24),
    centerX,
    start: { x: centerX, y: startY },
    end: { x: centerX, y: endY },
    connections: placement.connections,
    watcherBrackets: placement.watcherBrackets,
    nodes: placement.nodes,
    services: placement.services,
    sheets: placement.sheets,
    groups: placement.groups,
    loops: placement.loops,
    watchers: placement.watchers,
    selectedKey,
  };
}
