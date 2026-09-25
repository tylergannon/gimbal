// Sizing and persistence for the detail pane. Pure functions plus a thin
// localStorage wrapper — no Svelte state lives here.

export const DEFAULT_WIDTH = 480;
export const MIN_WIDTH = 380;
/** The map keeps at least this much width when the pane is not maximized. */
export const MIN_MAP_WIDTH = 360;

const WIDTH_KEY = "gimbal.detail.width";
const MAX_KEY = "gimbal.detail.max";

/** Clamp a candidate pane width to the drag range for the given viewport width. */
export function clampWidth(width: number, viewport: number): number {
  const max = Math.max(MIN_WIDTH, viewport - MIN_MAP_WIDTH);
  return Math.min(max, Math.max(MIN_WIDTH, width));
}

function readLocalStorage(key: string): string | null {
  if (typeof localStorage === "undefined") return null;
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeLocalStorage(key: string, value: string): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(key, value);
  } catch {
    // Storage may be unavailable (private browsing, quota, disabled) — the
    // pane still works, it just will not remember its size.
  }
}

/** Read the remembered pane width, falling back to the default when absent or invalid. */
export function readStoredWidth(): number {
  const raw = readLocalStorage(WIDTH_KEY);
  const parsed = raw === null ? NaN : Number(raw);
  return Number.isFinite(parsed) ? parsed : DEFAULT_WIDTH;
}

export function storeWidth(width: number): void {
  writeLocalStorage(WIDTH_KEY, String(width));
}

/** Read the remembered maximized flag, defaulting to false when absent or invalid. */
export function readStoredMaximized(): boolean {
  return readLocalStorage(MAX_KEY) === "true";
}

export function storeMaximized(maximized: boolean): void {
  writeLocalStorage(MAX_KEY, String(maximized));
}
