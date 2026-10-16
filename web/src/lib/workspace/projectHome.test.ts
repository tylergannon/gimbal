import { expect, test } from "vite-plus/test";
import type { Conversation } from "#lib/skgo/conversation/types";
import type { RunCardItem } from "#lib/run/RunsList.svelte";
import type { InterviewRow } from "#lib/observation";
import { projectActivity, runStatus } from "./projectHome";
const run = {
  run: { id: "run", name: "review", status: "running", error: "", started: 100, ended: 0 },
  activity_at: 200,
  session_count: 2,
  summary: "Testing checkout",
} as RunCardItem;
const question = {
  run: "run",
  status: "pending",
  question: "Which payment provider?",
} as InterviewRow;
test("project activity uses one honest status and sorts mixed records by actual change", () => {
  const session = {
    id: "session",
    title: "Build checkout",
    status: "working",
    updated: 300,
    provider: "codex",
    model: "gpt-5.6-luna",
    branch: "feature",
    requests: [],
    error: "",
    active_request: "request",
  } as unknown as Conversation;
  const entries = projectActivity("project", [session], [run], [question], []);
  expect(entries.map((item) => item.kind)).toEqual(["Sessions", "Runs"]);
  expect(entries[0].status).toBe("running");
  expect(entries[1]).toMatchObject({
    status: "needs-you",
    question: "Which payment provider?",
    bound: "2 sessions",
  });
  expect(entries[0].bound).toContain("Codex");
  expect(entries[1].path).toBe("/projects/project/runs/run");
  expect(runStatus({ ...run.run, status: "cancelled" }, [question])).toBe("cancelled");
  expect(runStatus({ ...run.run, status: "failed" }, [question])).toBe("failed");
  expect(runStatus({ ...run.run, status: "completed" }, [question])).toBe("ended");
});
