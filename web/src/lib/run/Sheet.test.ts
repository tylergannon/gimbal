import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { implementInterviewSnapshot } from "./fixtures/index.js";
import { countWrittenContextKeys, resolveScopeInstance } from "./Sheet.svelte";

const taskScopes = Object.values(implementInterviewSnapshot.scopes)
  .filter((scope) => scope.key.startsWith("implementation.1/task."))
  .sort((left, right) => left.began - right.began);

test("counts the runtime task key once when it is present in both fields", () => {
  const scope = taskScopes[2];

  assert.notEqual(scope.task, undefined);
  assert.ok("task" in scope.values);
  assert.equal(countWrittenContextKeys(scope), 1);
  assert.equal(countWrittenContextKeys(taskScopes[1]), 8);
});

test("defaults a repeated scope to its latest chronological instance", () => {
  assert.equal(resolveScopeInstance(taskScopes[0], taskScopes).key, taskScopes[2].key);
});
