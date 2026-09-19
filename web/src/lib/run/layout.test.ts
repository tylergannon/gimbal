import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";
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
