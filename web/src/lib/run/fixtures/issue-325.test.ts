import { expect, test } from "vite-plus/test";
import { graphMatchesSnapshot } from "../selection.js";
import { issue325Finished, issue325Fixture } from "./issue-325.js";

test("the issue 325 graph and snapshots are a pair the run page accepts", () => {
  expect(graphMatchesSnapshot(issue325Fixture.graph, issue325Fixture.snapshot)).toBe(true);
  expect(graphMatchesSnapshot(issue325Fixture.graph, issue325Finished)).toBe(true);
});
