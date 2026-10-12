import { expect, test } from "vite-plus/test";
import { issue325Finished } from "./fixtures/index.js";
import { turnOutcome, watcherRows } from "./watchers.js";
import type { AgentCall } from "../workflow/types.js";

test("watcher summaries preserve uncertainty, failure, interruption, and success separately", () => {
  const snapshot = structuredClone(issue325Finished);
  const row = snapshot.turns["coding.1/turn.2"];
  snapshot.turns = { [row.id]: row };
  const definition: AgentCall = {
    requirements: [],
    file: "workflow.go",
    line: 1,
    session: "worker",
    role: "coding",
    prompt: "work",
    supervisors: [
      {
        file: "workflow.go",
        line: 2,
        session: snapshot.sessions[row.session].name,
        role: "review",
        instruction: "watch",
        supervisors: [],
      },
    ],
  };
  for (const condition of [
    {
      unresolved: true,
      interrupted: false,
      error: "delivery lost after retained native success",
      ended: 2,
      verdict: "unresolved",
      pip: "unresolved",
    },
    {
      unresolved: false,
      interrupted: false,
      error: "native provider failure",
      ended: 2,
      verdict: "failed",
      pip: "failed",
    },
    {
      unresolved: false,
      interrupted: true,
      error: "context canceled",
      ended: 2,
      verdict: "interrupted",
      pip: "ended",
    },
    { unresolved: false, interrupted: false, error: "", ended: 2, verdict: "ended", pip: "ended" },
    {
      unresolved: false,
      interrupted: false,
      error: "",
      ended: 0,
      verdict: "running",
      pip: "running",
    },
  ]) {
    Object.assign(row, condition);
    expect(turnOutcome(row)).toBe(condition.verdict);
    const [watcher] = watcherRows(snapshot, row.scope, definition);
    expect(watcher.verdict).toBe(condition.verdict);
    expect(watcher.pip).toBe(condition.pip);
  }
});
