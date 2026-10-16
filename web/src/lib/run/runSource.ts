/**
 * The run map draws a run with the workflow designer's shapes. This module
 * joins the designer's SourcePage to the run: each ViewNode is matched to the
 * run graph operation at the same source position, and its declared scope
 * path is resolved to the runtime scope instance the viewer has chosen (or
 * the latest one). The result is the state, selection and instance choices
 * the map lays over the designer's drawing.
 */
import type { RunSnapshot, ScopeRow } from "../observation/index.js";
import type { SourcePage, ViewNode } from "../workflow-design/types.js";
import type { Graph, Service, Supervisor } from "../workflow/types.js";
import type { MapSelection } from "./types.js";
import type { NodeOperation } from "./types.js";
import type { PipState } from "./Pip.svelte";
import {
  childScopes,
  commandFor,
  instanceGroupKey,
  nodeSelectionKey,
  operationMeta,
  operationState,
  runtimeFor,
  serviceSelectionKey,
  sheetSelectionKey,
  turnFor,
  turnState,
  watcherSelectionKey,
} from "./layout.js";

type Operation = Graph["body"][number];
type GroupChild = Extract<Operation, { kind: "group" }>["children"][number];
type Matched = Operation | (GroupChild & { kind: "go" }) | (Service & { kind: "service" });

export type WatcherMark = {
  supervisor: Supervisor;
  state: PipState;
  selection: MapSelection;
  selectionKey: string;
};

export type RunMark = {
  state: PipState;
  meta: string;
  /** The runtime scope the node ran in, or its own instance for a container. */
  scopeKey?: string;
  selection?: MapSelection;
  selectionKey?: string;
  /** Instances a repeated container ran, oldest first, and the one shown. */
  instances?: ScopeRow[];
  instance?: string;
  instanceGroup?: string;
  watchers: WatcherMark[];
};

export type RunBinding = {
  marks: Map<string, RunMark>;
  /** Designer ids of containers and conditions holding started work. */
  active: Set<string>;
};

const position = (file: string, line: number) => `${file}:${line}`;

function indexGraph(graph: Graph) {
  const index = new Map<string, Matched[]>();
  const add = (item: Matched) => {
    const key = position(item.file, item.line);
    index.set(key, [...(index.get(key) ?? []), item]);
  };
  const services = (list: Service[]) => list.forEach((s) => add({ ...s, kind: "service" }));
  const walk = (operations: readonly unknown[]) => {
    for (const raw of operations) {
      const operation = raw as Operation;
      add(operation);
      if (operation.kind === "condition") operation.branches.forEach((b) => walk(b.body));
      if (operation.kind === "group")
        for (const child of operation.children) {
          add({ ...child, kind: "go" });
          services(child.services);
          walk(child.body);
        }
      if (
        operation.kind === "iterate" ||
        operation.kind === "scope" ||
        operation.kind === "promise_loop"
      ) {
        services(operation.services);
        walk(operation.body);
      }
      if (operation.kind === "repeat") walk(operation.body);
    }
  };
  services(graph.services);
  walk(graph.body);
  return index;
}

const graphKind: Record<string, string> = {
  Generate: "agent_call",
  Command: "command",
  Interview: "interview",
  PromiseLoop: "promise_loop",
  Iterate: "iterate",
  Scope: "scope",
  Group: "group",
  Go: "go",
  Repeat: "repeat",
  Condition: "condition",
  Service: "service",
  NewSession: "session",
  Set: "set",
  SetJSON: "set",
};

function scopeState(scope: ScopeRow | undefined): PipState {
  if (!scope) return "queued";
  if (scope.error) return "failed";
  return scope.status === "running" ? "running" : "ended";
}

const declared = (path: string) =>
  path
    .split("/")
    .filter(Boolean)
    .slice(path.startsWith("root") ? 1 : 0);

/**
 * Bind a SourcePage to a run. `selectedInstances` maps an instance group
 * (`parentKey/name`) to the runtime scope key the viewer picked.
 */
export function bindRun(
  page: SourcePage,
  graph: Graph,
  snapshot: RunSnapshot,
  selectedInstances: Readonly<Record<string, string>> = {},
): RunBinding {
  const index = indexGraph(graph);
  const marks = new Map<string, RunMark>();
  const active = new Set<string>();

  const pick = (parent: string, name: string) => {
    const instances = childScopes(snapshot, parent, name);
    const chosen = selectedInstances[instanceGroupKey(parent, name)];
    return { instances, scope: instances.find((s) => s.key === chosen) ?? instances.at(-1) };
  };
  /** The runtime scope key for a declared path, or undefined before it ran. */
  const resolve = (segments: string[]) => {
    let key = "";
    for (const name of segments) {
      const scope = pick(key, name).scope;
      if (!scope) return undefined;
      key = scope.key;
    }
    return key;
  };

  const match = (node: ViewNode) =>
    index
      .get(position(node.source.file, node.source.line))
      ?.find((item) => item.kind === graphKind[node.kind]);

  const watchersFor = (scopeKey: string, scope: ScopeRow, supervisors: Supervisor[]) =>
    supervisors.map((supervisor): WatcherMark => {
      const turn = turnFor(snapshot, scopeKey, supervisor.session);
      return {
        supervisor,
        state: turnState(turn),
        selection: { kind: "watcher", scope, supervisor, turn },
        selectionKey: watcherSelectionKey(scopeKey, supervisor),
      };
    });

  const bindStep = (operation: NodeOperation, scopeKey: string | undefined): RunMark => {
    const scope = scopeKey === undefined ? undefined : snapshot.scopes[scopeKey];
    if (scopeKey === undefined || !scope) return { state: "queued", meta: "", watchers: [] };
    const supervisors = operation.kind === "agent_call" ? operation.supervisors : [];
    return {
      state: operationState(snapshot, scopeKey, operation),
      meta: operationMeta(snapshot, scopeKey, operation),
      scopeKey,
      selection: {
        kind: "node",
        scope,
        operation,
        runtime: runtimeFor(snapshot, scopeKey, operation),
      },
      selectionKey: nodeSelectionKey(scopeKey, operation),
      watchers: watchersFor(scopeKey, scope, supervisors),
    };
  };

  const bindContainer = (parentKey: string | undefined, name: string): RunMark => {
    if (parentKey === undefined) return { state: "queued", meta: "", watchers: [] };
    const { instances, scope } = pick(parentKey, name);
    return {
      state: scopeState(scope),
      meta:
        instances.length > 1 && scope
          ? `${instances.indexOf(scope) + 1} of ${instances.length}`
          : "",
      scopeKey: scope?.key,
      selection: scope ? { kind: "sheet", scope } : undefined,
      selectionKey: scope ? sheetSelectionKey(scope.key) : undefined,
      instances,
      instance: scope?.key,
      instanceGroup: instanceGroupKey(parentKey, name),
      watchers: [],
    };
  };

  const bind = (node: ViewNode): RunMark | undefined => {
    const operation = match(node);
    if (!operation) return undefined;
    const path = declared(node.scope);
    switch (operation.kind) {
      case "agent_call":
      case "command":
      case "interview":
        return bindStep(operation, resolve(path));
      case "service": {
        const scopeKey = resolve(path);
        const scope = scopeKey === undefined ? undefined : snapshot.scopes[scopeKey];
        if (scopeKey === undefined || !scope) return { state: "queued", meta: "", watchers: [] };
        const runtime = commandFor(snapshot, scopeKey, operation.name);
        const service = { ...operation } as Service & { kind?: string };
        delete service.kind;
        return {
          state: !runtime
            ? "queued"
            : runtime.ended === 0
              ? "running"
              : runtime.error
                ? "failed"
                : "ended",
          meta: "",
          scopeKey,
          selection: { kind: "service", scope, service, runtime },
          selectionKey: serviceSelectionKey(scopeKey, service),
          watchers: [],
        };
      }
      case "go":
        return bindContainer(resolve(path.slice(0, -1)), path.at(-1) ?? operation.name);
      case "group":
      case "iterate":
      case "scope":
        return bindContainer(resolve(path), operation.name);
      case "promise_loop": {
        const loop = bindContainer(resolve(path), operation.name);
        const loopScope = loop.scopeKey === undefined ? undefined : snapshot.scopes[loop.scopeKey];
        if (loop.scopeKey === undefined || !loopScope) return loop;
        const tasks = pick(loop.scopeKey, "task");
        const planner = turnFor(snapshot, loop.scopeKey, operation.planner);
        const plannerState = turnState(planner);
        return {
          ...loop,
          // The loop is live while its planner or any task is.
          state: plannerState === "running" ? "running" : loop.state,
          meta: tasks.scope
            ? `task ${tasks.instances.indexOf(tasks.scope) + 1} of ${tasks.instances.length}`
            : "planning",
          instances: tasks.instances,
          instance: tasks.scope?.key,
          instanceGroup: instanceGroupKey(loop.scopeKey, "task"),
          watchers: watchersFor(loop.scopeKey, loopScope, operation.supervisors),
        };
      }
      default:
        return undefined;
    }
  };

  /** Bind a lane; returns whether anything in it has started. */
  const visit = (nodes: ViewNode[]): boolean => {
    let started = false;
    for (const node of nodes) {
      const mark = bind(node);
      if (mark) marks.set(node.id, mark);
      let inner = false;
      for (const lane of node.children ?? []) inner = visit(lane) || inner;
      const own = !!mark && mark.state !== "queued";
      if ((inner || own) && node.children?.length) active.add(node.id);
      started = started || inner || own;
    }
    return started;
  };
  visit(page.body);
  return { marks, active };
}

/** The selection key a MapSelection carries, matching RunMark.selectionKey. */
export function selectionKeyOf(selection: MapSelection | undefined) {
  if (!selection) return undefined;
  switch (selection.kind) {
    case "node":
      return nodeSelectionKey(selection.scope.key, selection.operation);
    case "service":
      return serviceSelectionKey(selection.scope.key, selection.service);
    case "watcher":
      return watcherSelectionKey(selection.scope.key, selection.supervisor);
    default:
      return sheetSelectionKey(selection.scope.key);
  }
}

/**
 * The designer page for a run saved before runs kept their source page. It
 * carries the same shapes the run graph records: no prose, titles or context.
 */
export function sourceFromGraph(graph: Graph): SourcePage {
  let next = 0;
  const node = (
    kind: string,
    label: string,
    at: { file: string; line: number },
    scope: string,
    extra: Partial<ViewNode> = {},
  ): ViewNode => ({
    id: `g${++next}`,
    kind,
    label,
    source: { file: at.file, line: at.line },
    scope,
    context: [],
    ...extra,
  });
  const services = (list: Service[], scope: string) =>
    list.map((s) => node("Service", s.name, s, scope));
  const lane = (operations: readonly unknown[], scope: string): ViewNode[] =>
    operations.flatMap((raw): ViewNode[] => {
      const op = raw as Operation;
      switch (op.kind) {
        case "agent_call":
          return [
            node("Generate", op.role || op.session, op, scope, {
              title: op.session,
              session: op.session,
              prompt: op.prompt,
            }),
          ];
        case "command":
          return [node("Command", op.name, op, scope)];
        case "interview":
          return [node("Interview", op.name, op, scope, { session: op.session })];
        case "session":
          return [node("NewSession", op.name, op, scope, { session: op.name })];
        case "set":
          return [node("Set", op.key, op, scope)];
        case "condition":
          return [
            node("Condition", "if / switch", op, scope, {
              control: { kind: "if", code: op.branches[0]?.case ?? "" },
              branchLabels: op.branches.map((b) => b.case),
              branchExits: op.branches.map((b) => b.exits),
              children: op.branches.map((b) => lane(b.body, scope)),
            }),
          ];
        case "repeat":
          return [
            node("Repeat", op.cond || "repeat", op, scope, { children: [lane(op.body, scope)] }),
          ];
        case "group": {
          const inner = `${scope}/${op.name}`;
          return [
            node("Group", op.name, op, scope, {
              children: op.children.map((child) => [
                node("Go", child.name, child, `${inner}/${child.name}`, {
                  children: [
                    [
                      ...services(child.services, `${inner}/${child.name}`),
                      ...lane(child.body, `${inner}/${child.name}`),
                    ],
                  ],
                }),
              ]),
            }),
          ];
        }
        case "iterate":
        case "scope": {
          const inner = `${scope}/${op.name}`;
          return [
            node(op.kind === "iterate" ? "Iterate" : "Scope", op.name, op, scope, {
              title: op.name,
              children: [[...services(op.services, inner), ...lane(op.body, inner)]],
            }),
          ];
        }
        case "promise_loop": {
          const inner = `${scope}/${op.name}/task`;
          return [
            node("PromiseLoop", op.name, op, scope, {
              title: op.name,
              session: op.planner,
              children: [[...services(op.services, inner), ...lane(op.body, inner)]],
            }),
          ];
        }
        default:
          return [];
      }
    });
  return {
    name: graph.name,
    entry: graph.name,
    module: "",
    source: graph.source,
    body: [...services(graph.services, "root"), ...lane(graph.body, "root")],
  };
}
