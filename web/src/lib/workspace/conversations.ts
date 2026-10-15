import type { Conversation } from "#lib/skgo/conversation/types";

/** Accepted prompts can still be waiting for async delivery while the chat is idle. */
export function conversationNeedsUpdates(item: Conversation): boolean {
  return (
    ["working", "stopping", "unresolved"].includes(item.status) ||
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
    if (!previous || item.updated > previous.updated) byID.set(item.id, item);
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
  seen?: string,
): "working" | "attention" | "unread" | undefined {
  if (item.status === "unresolved" || item.error) return "attention";
  if (
    ["working", "stopping"].includes(item.status) ||
    item.runs.some((run) => run.status === "running")
  )
    return "working";
  const reply = completedReply(item);
  return reply && reply !== seen ? "unread" : undefined;
}
