import type { JSONObject } from "#lib/sessionstate";

export type ActivityItem =
  | { id: string; type: "tools"; parts: JSONObject[] }
  | { id: string; type: "message"; message: JSONObject };

function toolKey(part: JSONObject): string {
  return String(part.name ?? "")
    .replace(/^.*(?:__|[./:])/, "")
    .replace(/([a-z])([A-Z])/g, "$1_$2")
    .toLowerCase();
}
export function isCommand(part: JSONObject): boolean {
  return ["bash", "command_execution", "exec_command", "shell", "run_command"].includes(
    toolKey(part),
  );
}

/** Action types come from known tool names, never from command text or a model. */
export function groupSummary(parts: JSONObject[]): string {
  const actions = new Map<string, string[]>();
  for (const part of parts) {
    const key = toolKey(part);
    const action = isCommand(part)
      ? "ran commands"
      : ["read", "read_file", "read_files"].includes(key)
        ? "read files"
        : ["write", "write_file", "edit", "apply_patch", "file_change"].includes(key)
          ? "edited files"
          : ["tool_search", "search_tools", "load_tool", "load_tools"].includes(key)
            ? "loaded tools"
            : ["web_search", "web_fetch", "browse", "browser"].includes(key) ||
                /^web[._]/.test(part.name ?? "")
              ? "used the browser"
              : key === "list_artifacts"
                ? "listed attached artifacts"
                : key === "create_worktree"
                  ? "created worktree"
                  : ["list_threads", "list_archived_threads"].includes(key)
                    ? "listed chats"
                    : "used tools";
    const states = actions.get(action) ?? [];
    states.push(toolStatus(part));
    actions.set(action, states);
  }
  const active: Record<string, string> = {
    "ran commands": "running commands",
    "read files": "reading files",
    "edited files": "editing files",
    "loaded tools": "loading tools",
    "used the browser": "using the browser",
    "listed attached artifacts": "listing attached artifacts",
    "created worktree": "creating worktree",
    "listed chats": "listing chats",
    "used tools": "using tools",
  };
  const summary = [...actions]
    .map(([action, states]) =>
      states.some((status) => ["running", "streaming"].includes(status))
        ? active[action]
        : states.every((status) => status === "pending")
          ? `queued ${action === "ran commands" ? "commands" : "tools"}`
          : action,
    )
    .join(", ");
  return summary ? summary[0].toUpperCase() + summary.slice(1) : "Used tools";
}
export function toolDetail(part: JSONObject): string {
  const input = part.state?.input;
  if (typeof input === "string") return input;
  return String(
    input?.command ??
      input?.cmd ??
      input?.file_path ??
      input?.path ??
      input?.query ??
      input?.pattern ??
      input?.description ??
      "",
  );
}
export function toolStatus(part: JSONObject): string {
  return part.state?.status ?? "pending";
}
export function toolVerb(part: JSONObject): string {
  const status = toolStatus(part);
  if (["running", "streaming"].includes(status)) return "Running";
  if (["error", "failed"].includes(status)) return "Failed";
  if (["canceled", "cancelled", "interrupted"].includes(status)) return "Stopped";
  if (status === "pending") return "Queued";
  return "Ran";
}
/** Presentation only: retain source identity/order; completion protocol is normalized upstream. */
export function turnPresentation(messages: JSONObject[], running: boolean) {
  const entries: Extract<ActivityItem, { type: "message" }>[] = messages.flatMap((message) => {
    if (message.type !== "assistant")
      return [{ id: String(message.id), type: "message" as const, message }];
    const content = message.content ?? [];
    const visible = content.filter(
      (part: JSONObject) => part.type !== "tool" || part.name !== "StructuredOutput",
    );
    if (
      visible.length < content.length &&
      !visible.some((part: JSONObject) => part.type !== "reasoning")
    )
      return [];
    return content.flatMap((part: JSONObject, index: number) => {
      if (
        (part.type === "tool" && part.name === "StructuredOutput") ||
        (part.type === "reasoning" && !part.text?.trim())
      )
        return [];
      return [
        {
          id: `${message.id}/${part.id ?? index}`,
          type: "message" as const,
          message: { ...message, error: undefined, retry: undefined, content: [part] },
        },
      ];
    });
  });
  const lastTool = entries.findLastIndex((entry) =>
    ["tool", "reasoning"].includes(entry.message.content?.[0]?.type),
  );
  const explicit = entries.findIndex(
    (entry) => entry.message.content?.[0]?.conversationDecision === true,
  );
  const fallback =
    !running && lastTool >= 0
      ? entries.findIndex(
          (entry, index) => index > lastTool && entry.message.content?.[0]?.type === "text",
        )
      : -1;
  const resultStart =
    explicit >= 0 ? explicit : fallback > lastTool ? fallback : lastTool < 0 ? 0 : entries.length;
  const activity: ActivityItem[] = [];
  for (const entry of entries.slice(0, resultStart)) {
    const part = entry.message.content?.[0];
    if (part?.type === "tool") {
      const previous = activity.at(-1);
      if (previous?.type === "tools") previous.parts.push({ ...part, id: entry.id });
      else activity.push({ id: entry.id, type: "tools", parts: [{ ...part, id: entry.id }] });
    } else activity.push(entry);
  }
  return { activity, results: entries.slice(resultStart) };
}
