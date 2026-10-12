import type { Conversation } from "#lib/skgo/conversation/types";

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
