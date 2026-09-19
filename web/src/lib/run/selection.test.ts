import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import {
  implementInterviewFixture,
  mismatchedHistoryFixture,
  planTripFixture,
  serviceOwnershipFixture,
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
  assert.equal(
    graphMatchesSnapshot(serviceOwnershipFixture.graph, serviceOwnershipFixture.snapshot),
    true,
    "service runtime command rows still belong to the static graph without becoming steps",
  );
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
  const waiting = currentActivitySelection(planTripFixture.graph, planTripFixture.snapshot);
  assert.equal(waiting?.kind, "node");
  if (waiting?.kind !== "node") return;
  assert.equal(waiting.scope.key, "research.1/transport.1");
  assert.equal(waiting.operation.kind, "interview");

  const active = currentActivitySelection(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
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

test("service selections rebind to refreshed owner scopes", () => {
  const operation = serviceOwnershipFixture.graph.body.find(
    (candidate) => candidate.kind === "scope",
  );
  if (!operation || operation.kind !== "scope") throw new Error("backend scope is missing");
  const service = operation.services[0];
  if (!service) throw new Error("api service is missing");
  const selection = {
    kind: "service" as const,
    scope: serviceOwnershipFixture.snapshot.scopes["backend.1"],
    service,
  };

  const refreshed = structuredClone(serviceOwnershipFixture.snapshot);
  const rebound = rebindSelection(selection, refreshed);
  assert.equal(rebound?.kind, "service");
  assert.equal(rebound?.scope, refreshed.scopes["backend.1"]);
  if (rebound?.kind === "service") assert.equal(rebound.service, service);
});

test("navigation keeps service process commands searchable as graph-backed service selections", () => {
  const items = runNavigationItems(serviceOwnershipFixture.graph, serviceOwnershipFixture.snapshot);
  const api = items.find((item) => item.id === "command:backend.1/api.1");
  assert.equal(api?.selection.kind, "service");
  if (api?.selection.kind !== "service") return;
  assert.equal(api.selection.service.name, "api");
  assert.equal(api.selection.scope.key, "backend.1");
  assert.equal(api.selection.runtime?.id, "backend.1/api.1");
});
