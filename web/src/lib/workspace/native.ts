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
