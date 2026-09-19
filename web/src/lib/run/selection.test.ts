import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import {
  implementInterviewFixture,
  mismatchedHistoryFixture,
  planTripFixture,
} from "./fixtures/index.js";
import { graphMatchesSnapshot } from "./selection.js";

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
