export type NativeStatus = {
  sessionID: string;
  sequence: number;
  pending: boolean;
  state: string;
  source: string;
  project: string;
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
