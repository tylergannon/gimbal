import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import {
  implementInterviewFixture,
  planTripFixture,
  serviceOwnershipFixture,
} from "./fixtures/index.js";
import { buildMapLayout } from "./layout.js";

const operationName = (
  operation: ReturnType<typeof buildMapLayout>["nodes"][number]["operation"],
) => (operation.kind === "agent_call" ? operation.session : operation.name);

test("lays out implement-interview recursively in source order", () => {
  const layout = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
  );

  assert.deepEqual(
    layout.nodes.map((node) => operationName(node.operation)),
    [
      "api-research",
      "frontend-research",
      "sprint-planning",
      "coding",
      "task-check",
      "build",
      "vet",
      "test",
      "qa-orchestration",
    ],
  );
  assert.deepEqual(
    layout.nodes.map((node) => node.state),
    ["ended", "ended", "ended", "running", "not-yet", "not-yet", "not-yet", "not-yet", "not-yet"],
  );
  assert.deepEqual(
    layout.nodes.slice(0, 2).map((node) => node.meta),
    ["", ""],
  );

  const group = layout.groups[0];
  assert.ok(group);
  assert.deepEqual(
    group.branches.map((branch) => branch.name),
    ["backend", "frontend"],
  );
  assert.equal(
    (group.branches[0].centerX + group.branches[1].centerX) / 2 + group.x,
    layout.centerX,
  );

  const loop = layout.loops[0];
  assert.ok(loop);
  assert.ok(loop.returnX > loop.bodyRight, "the return arrow owns a right margin");
  const finalBodyNode = layout.nodes.find(
    (node) => operationName(node.operation) === "qa-orchestration",
  );
  assert.ok(finalBodyNode);
  assert.equal(
    layout.connections.includes(
      `M${layout.centerX},${finalBodyNode.y + finalBodyNode.height} V${loop.y + loop.height}`,
    ),
    false,
    "the loop return and end connector remain separate",
  );
  assert.equal(layout.watchers.length, 2);
  assert.deepEqual(
    layout.watchers.map((watcher) => [watcher.supervisor.session, watcher.state]),
    [
      ["implementation-scope-review", "ended"],
      ["architectural-critique", "running"],
    ],
  );
  assert.equal(layout.watchers[1].y - layout.watchers[0].y, 48);
  assert.equal(
    layout.nodes.some((node) => operationName(node.operation) === "implementation-scope-review"),
    false,
    "watcher sessions are not drawn as steps",
  );
});

test("lays out plan-trip with pending interviews before its unexecuted planner", () => {
  const layout = buildMapLayout(planTripFixture.graph, planTripFixture.snapshot);

  assert.deepEqual(
    layout.nodes.map((node) => operationName(node.operation)),
    ["preferences", "preferences", "planner"],
  );
  assert.deepEqual(
    layout.nodes.map((node) => node.state),
    ["waiting", "waiting", "not-yet"],
  );
  assert.deepEqual(
    layout.nodes.slice(0, 2).map((node) => node.meta),
    ["question 2", "question 2"],
  );
  assert.equal(layout.nodes[2].meta, "not started");
  assert.equal(layout.nodes[2].x + layout.nodes[2].width / 2, layout.centerX);

  const group = layout.groups[0];
  assert.ok(group);
  assert.deepEqual(
    group.branches.map((branch) => branch.name),
    ["lodging", "transport"],
  );
  assert.equal(
    layout.sheets.some((sheet) => sheet.scope.name === "interviewer"),
    false,
  );
  assert.equal(layout.groups.length, 1);
  assert.equal(layout.loops.length, 0);
});

test("selected promise-loop instance drives every runtime fact", () => {
  const task2 = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    { selectedInstances: { "implementation.1/task": "implementation.1/task.2" } },
  );

  assert.equal(
    task2.sheets.find((sheet) => sheet.instances.length === 3)?.scope.key,
    "implementation.1/task.2",
  );
  assert.deepEqual(
    task2.nodes.slice(3).map((node) => [operationName(node.operation), node.state, node.meta]),
    [
      ["coding", "ended", "turn 2"],
      ["task-check", "failed", "exit 1"],
      ["build", "ended", "exit 0"],
      ["vet", "ended", "exit 0"],
      ["test", "ended", "exit 0"],
      ["qa-orchestration", "ended", "turn 1"],
    ],
  );

  const task3 = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    { selectedInstances: { "implementation.1/task": "implementation.1/task.3" } },
  );
  assert.deepEqual(
    task3.nodes.slice(3, 5).map((node) => [operationName(node.operation), node.state, node.meta]),
    [
      ["coding", "running", "turn 3"],
      ["task-check", "not-yet", "not started"],
    ],
  );
});

test("interrupted turns and commands are ended while genuine failures stay failed", () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  snapshot.turns["coding.1/turn.2"].interrupted = true;
  snapshot.turns["coding.1/turn.2"].error = "turn stopped";
  snapshot.commands["implementation.1/task.2/task-check.1"].interrupted = true;
  snapshot.commands["implementation.1/task.2/task-check.1"].error = "command stopped";
  snapshot.commands["implementation.1/task.2/build.1"].exit_code = 2;
  snapshot.commands["implementation.1/task.2/build.1"].error = "build failed";

  const layout = buildMapLayout(implementInterviewFixture.graph, snapshot, {
    selectedInstances: { "implementation.1/task": "implementation.1/task.2" },
  });
  const task2Nodes = layout.nodes.filter((node) => node.scopeKey === "implementation.1/task.2");

  assert.deepEqual(
    task2Nodes.map((node) => [operationName(node.operation), node.state]),
    [
      ["coding", "ended"],
      ["task-check", "ended"],
      ["build", "failed"],
      ["vet", "ended"],
      ["test", "ended"],
      ["qa-orchestration", "ended"],
    ],
  );
});

test("live folded elapsed time uses the render clock while ended and recorded scopes stay fixed", () => {
  const live = structuredClone(planTripFixture.snapshot);
  const started = live.run.started;
  const liveLayout = buildMapLayout(planTripFixture.graph, live, {
    foldedScopes: ["research.1"],
    now: started + 125_000,
  });
  assert.equal(
    liveLayout.sheets.find((sheet) => sheet.scope.key === "research.1")?.elapsed,
    "2m 00s",
  );

  const ended = structuredClone(live);
  ended.scopes["research.1"].ended = started + 65_000;
  const endedLayout = buildMapLayout(planTripFixture.graph, ended, {
    foldedScopes: ["research.1"],
    now: started + 999_000,
  });
  assert.equal(
    endedLayout.sheets.find((sheet) => sheet.scope.key === "research.1")?.elapsed,
    "1m 00s",
  );

  const recorded = structuredClone(live);
  recorded.run.status = "completed";
  recorded.run.ended = started + 125_000;
  recorded.scopes["research.1"].ended = 0;
  recorded.scopes["research.1/transport.1"].began = started + 900_000;
  const recordedLayout = buildMapLayout(planTripFixture.graph, recorded, {
    foldedScopes: ["research.1"],
    now: started + 999_000,
  });
  assert.equal(
    recordedLayout.sheets.find((sheet) => sheet.scope.key === "research.1")?.elapsed,
    "2m 00s",
  );
});

test("runtime ordinal scope names bind to their declared graph scopes", () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  for (const scope of Object.values(snapshot.scopes)) {
    if (scope.key) scope.name = scope.key.split("/").at(-1) ?? scope.name;
  }

  const layout = buildMapLayout(implementInterviewFixture.graph, snapshot);
  const implementation = layout.sheets.find((sheet) => sheet.scope.key === "implementation.1");
  const task = layout.sheets.find((sheet) => sheet.scope.key === "implementation.1/task.3");
  const coding = layout.nodes.find(
    (node) =>
      node.scopeKey === "implementation.1/task.3" && operationName(node.operation) === "coding",
  );

  assert.equal(implementation?.scope.name, "implementation.1");
  assert.equal(implementation?.kind, "loop");
  assert.equal(task?.scope.name, "task.3");
  assert.equal(task?.instances.length, 3);
  assert.equal(task?.instanceGroupKey, "implementation.1/task");
  assert.equal(
    coding?.runtime && "id" in coding.runtime ? coding.runtime.id : undefined,
    "coding.1/turn.3",
  );
  assert.equal(coding?.state, "running");
});

test("folded summaries retain child-scope states and selected-instance facts", () => {
  const reconnaissance = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    { foldedScopes: ["reconnaissance.1"] },
  ).sheets.find((sheet) => sheet.scope.key === "reconnaissance.1");
  assert.ok(reconnaissance);
  assert.deepEqual(
    reconnaissance.steps.map((step) => [operationName(step.operation), step.state]),
    [
      ["api-research", "ended"],
      ["frontend-research", "ended"],
    ],
  );

  const research = buildMapLayout(planTripFixture.graph, planTripFixture.snapshot, {
    foldedScopes: ["research.1"],
  }).sheets.find((sheet) => sheet.scope.key === "research.1");
  assert.ok(research);
  assert.deepEqual(
    research.steps.map((step) => [operationName(step.operation), step.state]),
    [
      ["preferences", "waiting"],
      ["preferences", "waiting"],
    ],
  );

  const task2 = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    {
      selectedInstances: { "implementation.1/task": "implementation.1/task.2" },
      foldedScopes: ["implementation.1/task.2"],
    },
  ).sheets.find((sheet) => sheet.scope.key === "implementation.1/task.2");
  assert.ok(task2);
  assert.deepEqual(
    task2.steps.slice(0, 2).map((step) => [operationName(step.operation), step.state]),
    [
      ["coding", "ended"],
      ["task-check", "failed"],
    ],
  );

  const implementation = buildMapLayout(
    implementInterviewFixture.graph,
    implementInterviewFixture.snapshot,
    {
      selectedInstances: { "implementation.1/task": "implementation.1/task.2" },
      foldedScopes: ["implementation.1"],
    },
  ).sheets.find((sheet) => sheet.scope.key === "implementation.1");
  assert.ok(implementation);
  assert.deepEqual(
    implementation.steps.slice(0, 3).map((step) => [operationName(step.operation), step.state]),
    [
      ["sprint-planning", "ended"],
      ["coding", "ended"],
      ["task-check", "failed"],
    ],
  );
});

test("a refreshed snapshot replaces selected-instance runtime facts", () => {
  const snapshot = structuredClone(implementInterviewFixture.snapshot);
  snapshot.turns["coding.1/turn.3"].ended = snapshot.turns["coding.1/turn.3"].started + 120_000;
  snapshot.turns["coding.1/turn.3"].duration = 120_000;
  snapshot.scopes["implementation.1/task.3"].status = "ended";
  snapshot.scopes["implementation.1/task.3"].ended = snapshot.turns["coding.1/turn.3"].ended;

  const layout = buildMapLayout(implementInterviewFixture.graph, snapshot, {
    selectedInstances: { "implementation.1/task": "implementation.1/task.3" },
  });
  const coding = layout.nodes.find(
    (node) =>
      node.scopeKey === "implementation.1/task.3" && operationName(node.operation) === "coding",
  );
  assert.equal(coding?.state, "ended");
});

test("services stay on their owning scopes and out of the operation sequence", () => {
  const layout = buildMapLayout(serviceOwnershipFixture.graph, serviceOwnershipFixture.snapshot);

  assert.deepEqual(
    layout.services.map((group) => [group.scopeKey, group.items.map((item) => item.service.name)]),
    [
      ["", ["root-db"]],
      ["backend.1", ["api"]],
      ["iteration.2", ["fixture"]],
    ],
  );
  assert.deepEqual(
    layout.nodes.map((node) => operationName(node.operation)),
    ["root-build", "build", "tests"],
  );

  for (const scopeKey of ["backend.1", "iteration.2"]) {
    const sheet = layout.sheets.find((candidate) => candidate.scope.key === scopeKey);
    const services = layout.services.find((candidate) => candidate.scopeKey === scopeKey);
    assert.ok(sheet);
    assert.ok(services);
    assert.ok(services.x >= sheet.x && services.x + services.width <= sheet.x + sheet.width);
    assert.ok(services.y >= sheet.y && services.y + services.height <= sheet.y + sheet.height);
  }

  for (const services of layout.services) {
    const center = services.x + services.width / 2;
    const crossesPanel = layout.connections.some((path) => {
      const match = /^M([\d.]+),([\d.]+) V([\d.]+)$/.exec(path);
      if (!match || Number(match[1]) !== center) return false;
      const from = Number(match[2]);
      const to = Number(match[3]);
      return from < services.y + services.height && to > services.y;
    });
    assert.equal(
      crossesPanel,
      false,
      `${services.scopeKey || "root"} has no sequence line through its service panel`,
    );
  }
});

test("folded owners retain a service count without rendering process rows", () => {
  const layout = buildMapLayout(serviceOwnershipFixture.graph, serviceOwnershipFixture.snapshot, {
    foldedScopes: ["iteration.2"],
  });
  const iteration = layout.sheets.find((sheet) => sheet.scope.key === "iteration.2");
  assert.ok(iteration);
  assert.equal(iteration.serviceCount, 1);
  assert.equal(
    layout.services.some((services) => services.scopeKey === "iteration.2"),
    false,
  );
});
