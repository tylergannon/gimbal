import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import {
  implementInterviewFixture,
  mismatchedHistoryFixture,
  planTripFixture,
} from "./fixtures/index.js";
import {
  currentActivitySelection,
  graphMatchesSnapshot,
  rebindSelection,
  runNavigationItems,
} from "./selection.js";

test("accepts runtime rows that fit the registered graph", () => {
  assert.equal(
    graphMatchesSnapshot(implementInterviewFixture.graph, implementInterviewFixture.snapshot),
    true,
  );
  assert.equal(graphMatchesSnapshot(planTripFixture.graph, planTripFixture.snapshot), true);
});

test("rejects a missing runtime path instead of inventing a map placement", () => {
  const snapshot = structuredClone(mismatchedHistoryFixture.snapshot);
  snapshot.scopes["implementation.1/unknown.1"] = {
    ...snapshot.scopes["implementation.1/task.4"],
    key: "implementation.1/unknown.1",
    name: "unknown",
  };
  assert.equal(graphMatchesSnapshot(implementInterviewFixture.graph, snapshot), false);
});

test("current activity resolves pending interviews and supervisor turns to their map nodes", () => {
  const waiting = currentActivitySelection(planTripFixture.graph, planTripFixture.snapshot, true);
  assert.equal(waiting?.kind, "node");
  if (waiting?.kind !== "node") return;
  assert.equal(waiting.scope.key, "research.1/transport.1");
  assert.equal(waiting.operation.kind, "interview");

  const active = currentActivitySelection(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    true,
  );
  assert.equal(active?.kind, "watcher");
  if (active?.kind !== "watcher") return;
  assert.equal(active.scope.key, "implementation.1/task.3");
  assert.equal(active.supervisor.session, "architectural-critique");
});

test("navigation finds old scope instances and turns with coordinated map selections", () => {
  const items = runNavigationItems(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    true,
  );
  const scope = items.find((item) => item.id === "scope:implementation.1/task.2");
  assert.equal(scope?.selection.kind, "sheet");
  assert.equal(scope?.selection.scope.key, "implementation.1/task.2");

  const turn = items.find((item) => item.id === "turn:coding.1/turn.2");
  assert.equal(turn?.selection.kind, "node");
  if (turn?.selection.kind !== "node") return;
  assert.equal(turn.selection.scope.key, "implementation.1/task.2");
  assert.equal(turn.selection.operation.kind, "agent_call");
  assert.ok(turn.selection.runtime && "id" in turn.selection.runtime);
  if (turn.selection.runtime && "id" in turn.selection.runtime) {
    assert.equal(turn.selection.runtime.id, "coding.1/turn.2");
  }
});

test("selection rebinds to refreshed rows and resets for missing items or another run", () => {
  const selected = runNavigationItems(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    true,
  ).find((item) => item.id === "turn:coding.1/turn.2")?.selection;
  assert.ok(selected);

  const refreshed = structuredClone(implementInterviewFixture.snapshot);
  const rebound = rebindSelection(selected, refreshed);
  assert.equal(rebound?.scope, refreshed.scopes["implementation.1/task.2"]);
  assert.equal(rebound?.kind, "node");
  if (rebound?.kind === "node") {
    assert.equal(rebound.runtime, refreshed.turns["coding.1/turn.2"]);
  }

  delete refreshed.turns["coding.1/turn.2"];
  assert.equal(rebindSelection(selected, refreshed), undefined);

  const anotherRun = structuredClone(implementInterviewFixture.snapshot);
  anotherRun.run.id = "another-run";
  assert.equal(rebindSelection(selected, anotherRun), undefined);
});
