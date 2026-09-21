import type { RunSnapshot, TurnRow } from "../observation/index.js";
import type { AgentCall } from "../workflow/types.js";
import type { WatcherRow } from "./detail/SessionDetail.svelte";

/** How a turn ended, in the words the inspector shows. */
export function turnOutcome(row: TurnRow): string {
  if (!row.ended) return "running";
  if (row.interrupted) return "interrupted";
  return row.error ? "failed" : "ended";
}

/** The latest turn in a scope of any session with this name. */
export function latestTurnOf(
  snapshot: RunSnapshot,
  scopeKey: string,
  sessionName: string,
): TurnRow | undefined {
  const sessionIDs = new Set(
    Object.values(snapshot.sessions)
      .filter((row) => row.name === sessionName)
      .map((row) => row.id),
  );
  return Object.values(snapshot.turns)
    .filter((row) => row.scope === scopeKey && sessionIDs.has(row.session))
    .sort((left, right) => left.started - right.started)
    .at(-1);
}

/** One row per watcher declared on an agent call: its latest turn in the
 * scope, or a placeholder when it has not run there yet. */
export function watcherRows(
  snapshot: RunSnapshot,
  scopeKey: string,
  definition: AgentCall | undefined,
): WatcherRow[] {
  return (
    definition?.supervisors.map((watcher) => {
      const watcherTurn = latestTurnOf(snapshot, scopeKey, watcher.session);
      if (!watcherTurn) {
        return {
          session: watcher.session,
          role: watcher.role,
          pip: "not-yet",
          verdict: "not yet run",
          result: "No watcher turn has been recorded in this scope.",
        };
      }
      return {
        session: watcher.session,
        role: watcher.role,
        pip: !watcherTurn.ended ? "running" : watcherTurn.error ? "failed" : "ended",
        verdict: turnOutcome(watcherTurn),
        result:
          watcherTurn.result ||
          watcherTurn.error ||
          "No completed watcher result has been recorded.",
      };
    }) ?? []
  );
}
