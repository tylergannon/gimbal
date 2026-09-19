import type {
  CommandRow,
  InterviewRow,
  RunSnapshot,
  ScopeRow,
  TurnRow,
} from "../observation/index.js";
import type { Graph, Service } from "../workflow/types.js";
import type { Supervisor } from "../workflow/types.js";
import type { MapSelection } from "./Map.svelte";
import { buildMapLayout } from "./layout.js";

export type RunSelection = MapSelection;

export type RunNavigationItem = {
  id: string;
  label: string;
  context: string;
  selection: RunSelection;
};

type Operation = Graph["body"][number];

type Shape = {
  scopes: Set<string>;
  calls: Set<string>;
  commands: Set<string>;
  interviews: Set<string>;
};

const join = (parent: string, name: string) => (parent ? `${parent}/${name}` : name);
const withoutOrdinals = (key: string) =>
  key
    .split("/")
    .filter(Boolean)
    .map((part) => part.replace(/\.\d+$/, ""))
    .join("/");

function addServices(shape: Shape, services: Service[], scope: string) {
  for (const service of services) shape.commands.add(join(scope, service.name));
}

function selectedInstancesFor(scopeKey: string) {
  const selected: Record<string, string> = {};
  let parent = "";
  for (const part of scopeKey.split("/")) {
    if (!part) continue;
    const key = parent ? `${parent}/${part}` : part;
    selected[`${parent}/${part.replace(/\.\d+$/, "")}`] = key;
    parent = key;
  }
  return selected;
}

function mapResolver(graph: Graph, snapshot: RunSnapshot) {
  const layouts = new Map<string, ReturnType<typeof buildMapLayout>>();
  const layoutFor = (scope: string) => {
    let layout = layouts.get(scope);
    if (!layout) {
      layout = buildMapLayout(graph, snapshot, { selectedInstances: selectedInstancesFor(scope) });
      layouts.set(scope, layout);
    }
    return layout;
  };

  return {
    turn(turn: TurnRow): MapSelection | undefined {
      const scope = snapshot.scopes[turn.scope];
      const session = snapshot.sessions[turn.session];
      if (!scope || !session) return undefined;
      const layout = layoutFor(turn.scope);
      const watcher = layout.watchers.find(
        (item) => item.watchedScope === turn.scope && item.supervisor.session === session.name,
      );
      if (watcher) {
        return { kind: "watcher", scope, supervisor: watcher.supervisor, turn };
      }
      const node = layout.nodes.find(
        (item) =>
          item.scopeKey === turn.scope &&
          ((item.operation.kind === "agent_call" && item.operation.session === session.name) ||
            (item.operation.kind === "interview" && item.operation.session === session.name)),
      );
      return node ? { kind: "node", scope, operation: node.operation, runtime: turn } : undefined;
    },
    interview(interview: InterviewRow): MapSelection | undefined {
      const scope = snapshot.scopes[interview.scope];
      const node = layoutFor(interview.scope).nodes.find(
        (item) =>
          item.scopeKey === interview.scope &&
          item.operation.kind === "interview" &&
          item.operation.name === interview.name,
      );
      return scope && node
        ? { kind: "node", scope, operation: node.operation, runtime: interview }
        : undefined;
    },
    command(command: CommandRow): MapSelection | undefined {
      const scope = snapshot.scopes[command.scope];
      const layout = layoutFor(command.scope);
      const node = layout.nodes.find(
        (item) =>
          item.scopeKey === command.scope &&
          item.operation.kind === "command" &&
          item.operation.name === command.name,
      );
      if (scope && node) {
        return { kind: "node", scope, operation: node.operation, runtime: command };
      }
      const declaredService = layout.services
        .find((group) => group.scopeKey === command.scope)
        ?.items.find((item) => item.service.name === command.name)?.service;
      return scope && declaredService
        ? { kind: "service", scope, service: declaredService, runtime: command }
        : undefined;
    },
  };
}

function addOperations(shape: Shape, operations: Operation[], scope: string) {
  for (const operation of operations) {
    switch (operation.kind) {
      case "agent_call":
        shape.calls.add(join(scope, operation.session));
        addSupervisors(shape, operation.supervisors, scope);
        break;
      case "command":
        shape.commands.add(join(scope, operation.name));
        break;
      case "interview":
        shape.interviews.add(join(scope, operation.name));
        // Interview exchanges are ordinary turns on the conducting session.
        shape.calls.add(join(scope, operation.session));
        break;
      case "scope":
      case "iterate": {
        const child = join(scope, operation.name);
        shape.scopes.add(child);
        addServices(shape, operation.services, child);
        addOperations(shape, operation.body as Operation[], child);
        break;
      }
      case "group": {
        const group = join(scope, operation.name);
        shape.scopes.add(group);
        for (const child of operation.children) {
          const branch = join(group, child.name);
          shape.scopes.add(branch);
          addServices(shape, child.services, branch);
          addOperations(shape, child.body as Operation[], branch);
        }
        break;
      }
      case "promise_loop": {
        const loop = join(scope, operation.name);
        const task = join(loop, "task");
        shape.scopes.add(loop);
        shape.scopes.add(task);
        shape.calls.add(join(loop, operation.planner));
        addSupervisors(shape, operation.supervisors, loop);
        addServices(shape, operation.services, task);
        addOperations(shape, operation.body as Operation[], task);
        break;
      }
      case "condition":
        for (const branch of operation.branches) {
          addOperations(shape, branch.body as Operation[], scope);
        }
        break;
      case "repeat":
        addOperations(shape, operation.body as Operation[], scope);
        break;
      case "session":
      case "set":
        break;
    }
  }
}

function addSupervisors(shape: Shape, supervisors: Supervisor[], scope: string) {
  for (const supervisor of supervisors) {
    // A supervisor's look turns are recorded in the scope of the call it watches.
    shape.calls.add(join(scope, supervisor.session));
    addSupervisors(shape, supervisor.supervisors, scope);
  }
}

/** Says whether every recorded runtime identity has an honest home in the
 * current compiled graph. It deliberately proves only structural compatibility:
 * the run record carries no graph version, so equal shape is not asserted to be
 * byte-for-byte historical identity. */
export function graphMatchesSnapshot(graph: Graph, snapshot: RunSnapshot) {
  if (graph.name !== snapshot.run.name) return false;
  const shape: Shape = {
    scopes: new Set([""]),
    calls: new Set(),
    commands: new Set(),
    interviews: new Set(),
  };
  addServices(shape, graph.services, "");
  addOperations(shape, graph.body, "");

  if (
    Object.values(snapshot.scopes).some((scope) => !shape.scopes.has(withoutOrdinals(scope.key)))
  ) {
    return false;
  }

  const sessions = snapshot.sessions;
  if (
    Object.values(snapshot.turns).some((turn) => {
      const session = sessions[turn.session];
      return !session || !shape.calls.has(join(withoutOrdinals(turn.scope), session.name));
    })
  ) {
    return false;
  }
  if (
    Object.values(snapshot.commands).some(
      (command) => !shape.commands.has(join(withoutOrdinals(command.scope), command.name)),
    )
  ) {
    return false;
  }
  return !Object.values(snapshot.interviews).some(
    (interview) => !shape.interviews.has(join(withoutOrdinals(interview.scope), interview.name)),
  );
}

export function selectedRuntimeKey(selection: RunSelection | undefined) {
  if (!selection) return undefined;
  if (selection.kind === "sheet" || selection.kind === "instance") {
    return selection.scope.key;
  }
  if (selection.kind === "watcher") return `watcher:${selection.supervisor.session}`;
  if (selection.kind === "service") {
    if (selection.runtime) return selection.runtime.id;
    return `service:${selection.scope.key}:${selection.service.name}:${selection.service.file}:${selection.service.line}`;
  }
  const runtime = selection.runtime;
  if (!runtime) return `${selection.scope.key}:${selection.operation.kind}`;
  if ("question_id" in runtime) return runtime.question_id;
  return runtime.id;
}

export function mapSelectionForTurn(
  graph: Graph,
  snapshot: RunSnapshot,
  turn: TurnRow,
): MapSelection | undefined {
  return mapResolver(graph, snapshot).turn(turn);
}

export function mapSelectionForInterview(
  graph: Graph,
  snapshot: RunSnapshot,
  interview: InterviewRow,
): MapSelection | undefined {
  return mapResolver(graph, snapshot).interview(interview);
}

function mapSelectionForCommand(
  graph: Graph,
  snapshot: RunSnapshot,
  command: CommandRow,
): MapSelection | undefined {
  return mapResolver(graph, snapshot).command(command);
}

export function currentActivitySelection(
  graph: Graph,
  snapshot: RunSnapshot,
): RunSelection | undefined {
  const interview = Object.values(snapshot.interviews)
    .filter((row) => row.status === "pending")
    .sort((left, right) => right.asked - left.asked)[0];
  if (interview) {
    return mapSelectionForInterview(graph, snapshot, interview);
  }

  const turn = Object.values(snapshot.turns)
    .filter((row) => row.ended === 0)
    .sort((left, right) => right.started - left.started)[0];
  if (!turn) return undefined;
  return mapSelectionForTurn(graph, snapshot, turn);
}

export function runNavigationItems(graph: Graph, snapshot: RunSnapshot): RunNavigationItem[] {
  const items: RunNavigationItem[] = [];
  const resolver = mapResolver(graph, snapshot);

  for (const scope of Object.values(snapshot.scopes).sort(
    (left, right) => left.began - right.began || left.key.localeCompare(right.key),
  )) {
    if (!scope.key) continue;
    items.push({
      id: `scope:${scope.key}`,
      label: scope.key,
      context: scope.loop ? "loop scope" : "scope",
      selection: { kind: "sheet", scope },
    });
  }

  for (const turn of Object.values(snapshot.turns).sort(
    (left, right) => left.started - right.started || left.id.localeCompare(right.id),
  )) {
    const scope = snapshot.scopes[turn.scope];
    const session = snapshot.sessions[turn.session];
    if (!scope) continue;
    const mapSelection = resolver.turn(turn);
    if (!mapSelection) continue;
    const ordinal = /turn\.(\d+)$/.exec(turn.id)?.[1];
    items.push({
      id: `turn:${turn.id}`,
      label: `${session?.name ?? turn.session}${ordinal ? ` · turn ${ordinal}` : ""}`,
      context: turn.scope || snapshot.run.name,
      selection: mapSelection,
    });
  }

  for (const command of Object.values(snapshot.commands).sort(
    (left, right) => left.started - right.started || left.id.localeCompare(right.id),
  )) {
    const scope = snapshot.scopes[command.scope];
    if (!scope) continue;
    const mapSelection = resolver.command(command);
    if (!mapSelection) continue;
    items.push({
      id: `command:${command.id}`,
      label: command.name,
      context: command.scope || snapshot.run.name,
      selection: mapSelection,
    });
  }

  return items;
}

export function rebindSelection(
  selection: RunSelection | undefined,
  snapshot: RunSnapshot,
): RunSelection | undefined {
  if (!selection || selection.scope.run !== snapshot.run.id) return undefined;
  const scope = snapshot.scopes[selection.scope.key];
  if (!scope) return undefined;

  if (selection.kind === "sheet" || selection.kind === "instance") {
    return { ...selection, scope };
  }
  if (selection.kind === "watcher") {
    if (!selection.turn) return { ...selection, scope };
    const turn = snapshot.turns[selection.turn.id];
    return turn ? { ...selection, scope, turn } : undefined;
  }
  if (selection.kind === "service") {
    if (!selection.runtime) return { ...selection, scope };
    const runtime = snapshot.commands[selection.runtime.id];
    return runtime ? { ...selection, scope, runtime } : undefined;
  }

  if (!selection.runtime) return { ...selection, scope };
  let runtime: TurnRow | CommandRow | InterviewRow | undefined;
  if ("question_id" in selection.runtime) {
    runtime = snapshot.interviews[selection.runtime.question_id];
  } else if ("command" in selection.runtime) {
    runtime = snapshot.commands[selection.runtime.id];
  } else {
    runtime = snapshot.turns[selection.runtime.id];
  }
  return runtime ? { ...selection, scope, runtime } : undefined;
}

export function asMapSelection(selection: RunSelection | undefined): MapSelection | undefined {
  return selection;
}
