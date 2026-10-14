export type NativeStatus = {
  sessionID: string;
  sequence: number;
  pending: boolean;
  state: string;
  source: string;
  project: string;
  elapsedMs?: number;
  requirements?: boolean;
  canPause?: boolean;
  canResume?: boolean;
  canStop?: boolean;
  switching?: boolean;
  error?: string;
};
declare global {
  interface Window {
    voiceNotesLatestStatus?: NativeStatus;
    voiceNotesNativeStatus?: (status: NativeStatus) => void;
    webkit?: { messageHandlers?: Record<string, { postMessage: (value: unknown) => void }> };
  }
}
export function nativeMessage(name: string, value: unknown) {
  window.webkit?.messageHandlers?.[name]?.postMessage(value);
}
// The desktop answers with a `gimbal-project-folder` event: the chosen path, or null when cancelled.
export function chooseProjectFolder(): Promise<string | undefined> {
  if (!window.webkit?.messageHandlers?.chooseProjectFolder) return Promise.resolve(undefined);
  return new Promise((resolve) => {
    window.addEventListener(
      "gimbal-project-folder",
      (event) => resolve((event as CustomEvent<{ path: string | null }>).detail.path ?? undefined),
      { once: true },
    );
    nativeMessage("chooseProjectFolder", {});
  });
}

export function recordingTime(durationMs: number | null | undefined): string {
  if (durationMs == null) return "Duration unavailable";
  const seconds = Math.floor(durationMs / 1000);
  const pair = (value: number) => String(value).padStart(2, "0");
  return seconds >= 3600
    ? `${Math.floor(seconds / 3600)}:${pair(Math.floor(seconds / 60) % 60)}:${pair(seconds % 60)}`
    : `${pair(Math.floor(seconds / 60))}:${pair(seconds % 60)}`;
}
export function projectMenu(event: MouseEvent) {
  const anchor = (event.currentTarget as HTMLElement).getBoundingClientRect();
  nativeMessage("projectMenu", { x: anchor.left, y: anchor.bottom });
}
export function recordingControl(status: NativeStatus, action: "pause" | "resume" | "stop") {
  nativeMessage("recordingControl", {
    sessionID: status.sessionID,
    project: status.project,
    action,
  });
}
