/** Display data only; the parent owns work, recording and tab lifetimes. */
export interface WorkspaceTab {
  id: string;
  title: string;
  kind: "chat" | "file";
  status?: "working" | "attention" | "unread";
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
