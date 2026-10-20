import type { Conversation } from "#lib/skgo/conversation/types";

/** Accepted prompts can still be waiting for async delivery while the chat is idle. */
export function conversationNeedsUpdates(item: Conversation): boolean {
  return (
    ["working", "stopping", "unresolved", "runnable", "waiting"].includes(item.status) ||
    item.requests.some((request) => request.status === "accepted") ||
    item.runs.some((run) => run.status === "running")
  );
}

/** Route loads and query refreshes are both factual; keep the newest record. */
export function conversationsForWorkspace(
  inventory: Conversation[],
  route: Conversation[],
  selected?: Conversation,
): Conversation[] {
  const byID = new Map(inventory.map((item) => [item.id, item]));
  for (const item of [...route, ...(selected?.id ? [selected] : [])]) {
    const previous = byID.get(item.id);
    if (!previous || item.revision > previous.revision || item.updated > previous.updated)
      byID.set(item.id, item);
  }
  return [...byID.values()];
}

/** A steer acknowledgment is not a completed agent reply. */
export function completedReply(item: Conversation): string {
  return (
    item.requests.findLast(
      (request) =>
        request.kind === "prompt" && request.status === "completed" && request.delivery === "turn",
    )?.id ?? ""
  );
}

/** Recovery and errors need a person; an unresolved outcome is not active work. */
export function conversationStatus(
  item: Conversation,
): "queued" | "running" | "needs-you" | "ended" | "failed" | "cancelled" {
  if (item.status === "unresolved") return "needs-you";
  if (item.error || ["failed", "error"].includes(item.status)) return "failed";
  if (["working", "stopping", "runnable"].includes(item.status)) return "running";
  if (item.requests.some((request) => request.status === "accepted")) return "queued";
  if (item.status === "canceled" || item.status === "cancelled") return "cancelled";
  if (item.status === "waiting") return "needs-you";
  return "ended";
}
