import type { CommandRow, InteractiveRow, ScopeRow, TurnRow } from "../observation/index.js";
import type { AgentCall, Command, Interactive, Service, Supervisor } from "../workflow/types.js";

/** The run graph operations the map draws as steps with a runtime record. */
export type NodeOperation =
  | (AgentCall & { kind: "agent_call" })
  | (Command & { kind: "command" })
  | (Interactive & { kind: "interactive" });

/** What the run map selects and the detail pane shows. */
export type MapSelection =
  | { kind: "sheet"; scope: ScopeRow }
  | { kind: "instance"; scope: ScopeRow }
  | {
      kind: "node";
      scope: ScopeRow;
      operation: NodeOperation;
      runtime?: TurnRow | CommandRow | InteractiveRow;
    }
  | { kind: "service"; scope: ScopeRow; service: Service; runtime?: CommandRow }
  | {
      kind: "watcher";
      scope: ScopeRow;
      supervisor: Supervisor;
      turn?: TurnRow;
    };
