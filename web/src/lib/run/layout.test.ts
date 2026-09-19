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
