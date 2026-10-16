import { expect, test } from "vite-plus/test";
import type { ViewNode } from "../workflow-design/types.js";
import { implementInterviewFixture, planTripFixture } from "./fixtures/index.js";
import { bindRun, selectionKeyOf, sourceFromGraph } from "./runSource.js";

const { graph, snapshot } = implementInterviewFixture;

function find(nodes: ViewNode[], test: (node: ViewNode) => boolean): ViewNode | undefined {
  for (const node of nodes) {
    if (test(node)) return node;
    for (const lane of node.children ?? []) {
      const found = find(lane, test);
      if (found) return found;
    }
  }
  return undefined;
}

test("a step binds to its latest runtime instance and record", () => {
  const source = sourceFromGraph(graph);
  const coding = find(source.body, (n) => n.kind === "Generate" && n.session === "coding")!;
  expect(coding.scope).toBe("root/implementation/task");

  const mark = bindRun(source, graph, snapshot).marks.get(coding.id)!;
  expect(mark.scopeKey).toBe("implementation.1/task.3");
  expect(mark.selection?.kind).toBe("node");
  expect(
    mark.selection &&
      "runtime" in mark.selection &&
      (mark.selection.runtime as { id?: string } | undefined)?.id,
  ).toBe("coding.1/turn.3");
  expect(mark.selectionKey).toBe(selectionKeyOf(mark.selection));
});

test("a chosen loop instance rebinds the steps inside it", () => {
  const source = sourceFromGraph(graph);
  const loop = find(source.body, (n) => n.kind === "PromiseLoop")!;
  const coding = find(source.body, (n) => n.kind === "Generate" && n.session === "coding")!;

  const latest = bindRun(source, graph, snapshot);
  expect(latest.marks.get(loop.id)?.instances?.map((s) => s.key)).toEqual([
    "implementation.1/task.1",
    "implementation.1/task.2",
    "implementation.1/task.3",
  ]);
  expect(latest.marks.get(loop.id)?.meta).toBe("task 3 of 3");
  expect(latest.active.has(loop.id)).toBe(true);

  const group = latest.marks.get(loop.id)!.instanceGroup!;
  const first = bindRun(source, graph, snapshot, { [group]: "implementation.1/task.1" });
  expect(first.marks.get(loop.id)?.meta).toBe("task 1 of 3");
  expect(first.marks.get(coding.id)?.scopeKey).toBe("implementation.1/task.1");
  expect(first.marks.get(coding.id)?.state).toBe("ended");
});

test("supervisors bind as watchers of the step they watch", () => {
  const source = sourceFromGraph(graph);
  const marks = bindRun(source, graph, snapshot).marks;
  const watched = [...marks.values()].filter((mark) => mark.watchers.length > 0);
  expect(watched.length).toBeGreaterThan(0);
  for (const mark of watched)
    for (const watcher of mark.watchers) {
      expect(watcher.selection.kind).toBe("watcher");
      expect(watcher.selectionKey).toBe(selectionKeyOf(watcher.selection));
    }
});

test("group branches resolve their own instances; unmatched designer nodes stay unbound", () => {
  const source = sourceFromGraph(planTripFixture.graph);
  const preferences = find(
    source.body,
    (n) => n.kind === "Interview" && n.scope === "root/research/transport",
  )!;
  const marks = bindRun(source, planTripFixture.graph, planTripFixture.snapshot).marks;
  expect(marks.get(preferences.id)?.scopeKey).toBe("research.1/transport.1");

  const moved = { ...preferences, id: "moved", source: { ...preferences.source, line: 9999 } };
  const page = { ...source, body: [...source.body, moved] };
  expect(bindRun(page, planTripFixture.graph, planTripFixture.snapshot).marks.has("moved")).toBe(
    false,
  );
});

test("steps in scopes that have not started are not yet", () => {
  const source = sourceFromGraph(graph);
  const empty = structuredClone(snapshot);
  for (const key of Object.keys(empty.scopes)) if (key) delete empty.scopes[key];
  empty.turns = {};
  empty.commands = {};
  const marks = bindRun(source, graph, empty).marks;
  const coding = find(source.body, (n) => n.kind === "Generate" && n.session === "coding")!;
  expect(marks.get(coding.id)?.state).toBe("not-yet");
  expect(marks.get(coding.id)?.selection).toBeUndefined();
});
