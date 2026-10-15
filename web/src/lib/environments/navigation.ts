import { goto } from "$app/navigation";

export function openMachines(project = "") {
  const native = (
    window as Window & {
      webkit?: {
        messageHandlers?: { openSettings?: { postMessage: (body: { project: string }) => void } };
      };
    }
  ).webkit?.messageHandlers?.openSettings;
  if (native) native.postMessage({ project });
  else
    void goto(
      project ? `/settings/machines/projects/${encodeURIComponent(project)}` : "/settings/machines",
    );
}

export function originIdentity(origin: string): string {
  const normalized = origin
    .trim()
    .replace(/^[^@/]+@([^:]+):/, "$1/")
    .replace(/^[a-z]+:\/\//i, "")
    .replace(/^[^@/]+@/, "")
    .replace(/\.git\/?$/, "")
    .replace(/\/$/, "");
  const slash = normalized.indexOf("/");
  return slash < 0
    ? normalized
    : normalized.slice(0, slash).toLowerCase() + normalized.slice(slash);
}

export function timeLabel(value: number): string {
  return value
    ? new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : "unknown time";
}
