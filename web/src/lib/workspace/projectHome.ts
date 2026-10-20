import type { Conversation } from "#lib/skgo/conversation/types";
import type { Header } from "#lib/skgo/transcript/types";
import type { RunCardItem } from "#lib/run/RunsList.svelte";
import type { InteractiveRow, RunRow } from "#lib/observation";
import type { WorkspaceTab } from "#lib/shell/types";
import { conversationStatus } from "./conversations";
import { harnessName, modelName } from "#lib/conversation/names";

export type HomeEntry = {
  path: string;
  title: string;
  kind: "Sessions" | "Runs" | "Recordings";
  status: NonNullable<WorkspaceTab["status"]>;
  bound: string;
  updated: number;
  detail: string;
  question?: string;
};
export const runStatus = (run: RunRow, attention: InteractiveRow[]): HomeEntry["status"] =>
  run.status === "running"
    ? attention.some((question) => question.run === run.id && question.status === "pending")
      ? "needs-you"
      : "running"
    : run.status === "failed"
      ? "failed"
      : run.status === "cancelled"
        ? "cancelled"
        : "ended";
export const statusLabel = (status: HomeEntry["status"]) =>
  ({
    queued: "Queued",
    running: "Running",
    "needs-you": "Needs you",
    ended: "Ended",
    failed: "Failed",
    cancelled: "Cancelled",
  })[status];
export function projectActivity(
  project: string,
  sessions: Conversation[],
  runs: RunCardItem[],
  attention: InteractiveRow[],
  recordings: Header[],
): HomeEntry[] {
  const root = `/projects/${project}`;
  // A conversation's controller waits for the same person as its session.
  // Child workflows remain separate work; only its own backing run is hidden.
  const controllers = new Set(sessions.map((item) => item.workflow_run).filter(Boolean));
  return [
    ...sessions.map((item) => ({
      path: `${root}/conversations/${item.id}`,
      title: item.title,
      kind: "Sessions" as const,
      status: conversationStatus(item),
      bound: [
        harnessName(item.provider),
        modelName(item.model),
        item.branch,
        item.placement?.machine.name,
      ]
        .filter(Boolean)
        .join(" · "),
      updated: item.updated,
      detail:
        item.error ||
        (item.active_request
          ? `turn ${item.requests.filter((request) => request.delivery === "turn").length}`
          : ""),
    })),
    ...runs.filter((item) => !controllers.has(item.run.id)).map((item) => ({
      path: `${root}/runs/${encodeURIComponent(item.run.id)}`,
      title: `${item.run.name} · ${item.run.id.split(".")[0].slice(-6)}`,
      kind: "Runs" as const,
      status: runStatus(item.run, attention),
      bound: [
        `${item.session_count} ${item.session_count === 1 ? "session" : "sessions"}`,
        item.run.task_environment,
      ]
        .filter(Boolean)
        .join(" · "),
      updated: item.activity_at || item.run.ended || item.run.started,
      detail: item.run.error || item.summary,
      question: attention.find(
        (question) =>
          question.run === item.run.id &&
          question.status === "pending" &&
          item.run.status === "running",
      )?.question,
    })),
    ...recordings.map((item) => ({
      path: `${root}/recordings/${item.id}`,
      title: `Recording · ${new Date(item.created).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}`,
      kind: "Recordings" as const,
      status: (item.status === "recording"
        ? "running"
        : item.status === "preparing"
          ? "queued"
          : item.status === "interrupted"
            ? "cancelled"
            : "ended") as HomeEntry["status"],
      bound: item.source,
      updated: item.updated || item.created,
      detail: item.status,
    })),
  ].sort((a, b) => b.updated - a.updated);
}
