import type { CommandRow, RunSnapshot, ScopeRow, TurnRow } from "../observation/index.js";
import type { Graph, Service } from "../workflow/types.js";
import type { Supervisor } from "../workflow/types.js";
import type { MapSelection } from "./Map.svelte";

export type HistorySelection =
  | { kind: "history-scope"; scope: ScopeRow }
  | { kind: "history-turn"; scope: ScopeRow; turn: TurnRow }
  | { kind: "history-command"; scope: ScopeRow; command: CommandRow };

export type RunSelection = MapSelection | HistorySelection;

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
  if (selection.kind === "history-turn") return selection.turn.id;
  if (selection.kind === "history-command") return selection.command.id;
  if (
    selection.kind === "history-scope" ||
    selection.kind === "sheet" ||
    selection.kind === "instance"
  ) {
    return selection.scope.key;
  }
  if (selection.kind === "watcher") return `watcher:${selection.supervisor.session}`;
  if (selection.kind === "service") {
    return `service:${selection.scope.key}:${selection.service.name}:${selection.service.file}:${selection.service.line}`;
  }
  const runtime = selection.runtime;
  if (!runtime) return `${selection.scope.key}:${selection.operation.kind}`;
  if ("question_id" in runtime) return runtime.question_id;
  return runtime.id;
}
