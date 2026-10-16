/** Display data only; the parent owns work, recording and tab lifetimes. */
export interface WorkspaceTab {
  id: string;
  title: string;
  kind: "chat" | "run" | "runs" | "file" | "recording" | "promises" | "guide";
  status?: "queued" | "running" | "needs-you" | "ended" | "failed" | "cancelled";
  unseen?: boolean;
  count?: number;
  origin?: { id: string; title: string };
  workflow?: string;
  members?: WorkspaceTab[];
  detail?: string;
}

export interface RecordingDisplay {
  state: "recording" | "paused" | "saving" | "error";
  label: string;
  owner?: string;
  detail?: string;
  canPause?: boolean;
  canResume?: boolean;
  canStop?: boolean;
}
