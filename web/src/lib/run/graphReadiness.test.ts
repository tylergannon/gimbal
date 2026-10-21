import { expect, test } from "vite-plus/test";
import { GraphReadiness } from "./graphReadiness";

test("readiness resolves only when the current layout is applied", async () => {
  const graph = new GraphReadiness();
  const revision = graph.begin();
  let settled = false;
  const ready = graph.whenReady().then((value) => {
    settled = true;
    return value;
  });
  graph.applied(revision - 1, 99, 99);
  await Promise.resolve();
  expect(settled).toBe(false);
  graph.applied(revision, 3, 2);
  await expect(ready).resolves.toEqual({ revision, nodes: 3, edges: 2 });
});

test("updates invalidate ready state and reject superseded waiters", async () => {
  const graph = new GraphReadiness();
  graph.applied(graph.begin(), 1, 0);
  const old = await graph.whenReady();
  graph.begin();
  const superseded = expect(graph.whenReady()).rejects.toThrow("superseded");
  const current = graph.begin();
  await superseded;
  graph.applied(old.revision, 99, 99);
  const ready = graph.whenReady();
  graph.applied(current, 4, 3);
  await expect(ready).resolves.toEqual({ revision: current, nodes: 4, edges: 3 });
});

test("layout errors reject readiness and a new revision can recover", async () => {
  const graph = new GraphReadiness();
  const revision = graph.begin();
  const rejected = expect(graph.whenReady()).rejects.toThrow("layout failed");
  graph.failed(revision, new Error("layout failed"));
  await rejected;
  await expect(graph.whenReady()).rejects.toThrow("layout failed");
  const next = graph.begin();
  graph.failed(revision, new Error("stale failure"));
  graph.applied(next, 2, 1);
  await expect(graph.whenReady()).resolves.toEqual({ revision: next, nodes: 2, edges: 1 });
});

test("unmount rejects waiting and subsequent callers", async () => {
  const graph = new GraphReadiness();
  graph.begin();
  const rejected = expect(graph.whenReady()).rejects.toThrow("unmounted");
  graph.dispose();
  await rejected;
  await expect(graph.whenReady()).rejects.toThrow("unmounted");
});
