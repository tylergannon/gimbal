import { expect, test } from "vite-plus/test";
import type { Conversation } from "#lib/skgo/conversation/types";
import {
  conversationsForWorkspace,
  conversationNeedsUpdates,
  completedReply,
  conversationStatus,
} from "./conversations";

const chat = (id: string, updated: number): Conversation => ({
  id,
  updated,
  title: id,
  provider: "codex",
  model: "gpt-5.6-luna",
  effort: "",
  shared_checkout: true,
  base: "",
  branch: "",
  worktree: "/project",
  native_session: "",
  live: true,
  status: "idle",
  error: "",
  created: 1,
  messages: [],
  runs: [],
  requests: [],
  active_request: "",
  state: "",
  revision: 0,
  environment: "local",
  placement: null,
  contact: "connected",
  capabilities: { steer: true, fork: true, worktree: true, browse: true, clone: true },
  cleanup_pending: false,
});

test("a newly routed chat remains available while inventory refresh lags", () => {
  const previous = chat("previous", 1);
  const created = chat("newly-created", 2);
  const items = conversationsForWorkspace([previous], [previous, created], created);
  expect(items.map((item) => item.id)).toEqual(["previous", "newly-created"]);
  expect(items.find((item) => item.id === created.id)).toBe(created);
  // Even a route that carries only its selected record must open that real chat.
  expect(conversationsForWorkspace([previous], [], created)).toContain(created);
});

test("query progress and freshly loaded route messages each retain the newer data", () => {
  const old = chat("existing", 1);
  const working = {
    ...chat("existing", 2),
    status: "working",
    messages: [{ role: "user", text: "Start", created: 2 }],
  };
  expect(conversationsForWorkspace([working], [old], old)[0]).toBe(working);
  const completed = {
    ...working,
    updated: 3,
    status: "idle",
    messages: [...working.messages, { role: "assistant", text: "Done", created: 3 }],
  };
  expect(conversationsForWorkspace([working], [completed], completed)[0]).toBe(completed);
});

test("an accepted prompt keeps observation updating before async delivery changes idle state", () => {
  const accepted = chat("accepted-before-delivery", 2);
  accepted.requests = [
    {
      id: "request",
      created: 2,
      kind: "prompt",
      message: "Start",
      target: "",
      file_id: "",
      status: "accepted",
      delivery: "",
      error: "",
    },
  ];
  expect(accepted.status).toBe("idle");
  expect(accepted.active_request).toBe("");
  expect(conversationNeedsUpdates(accepted)).toBe(true);
  accepted.requests[0].delivery = "queued";
  expect(conversationNeedsUpdates(accepted)).toBe(true);
  accepted.requests[0].status = "completed";
  expect(conversationNeedsUpdates(accepted)).toBe(false);
});

test("a later steer acknowledgment cannot hide the original turn completion", () => {
  const item = chat("steered", 3);
  const request = {
    id: "original",
    kind: "prompt",
    status: "completed",
    delivery: "turn",
  } as Conversation["requests"][number];
  item.requests = [request, { ...request, id: "steer", delivery: "steer" }];
  expect(completedReply(item)).toBe("original");
  item.requests = [
    { ...request, status: "working" },
    { ...request, id: "steer", delivery: "steer" },
  ];
  expect(completedReply(item)).toBe("");
});

test("restart uncertainty needs attention while real work and unread replies remain distinct", () => {
  const item = chat("recover", 1);
  item.status = "unresolved";
  item.error = "The server restarted; outcome unconfirmed";
  expect(conversationStatus(item)).toBe("needs-you");
  item.error = "";
  item.status = "working";
  expect(conversationStatus(item)).toBe("running");
  item.status = "idle";
  item.requests = [
    {
      id: "reply",
      kind: "prompt",
      status: "completed",
      delivery: "turn",
    } as Conversation["requests"][number],
  ];
  expect(conversationStatus(item)).toBe("ended");
  expect(completedReply(item)).toBe("reply");
});
