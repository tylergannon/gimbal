// A stand-in for the run page's EventSource in Storybook: it opens at once,
// so the page shows Live, and a story can push deltas through `send`.
import type { ObservationDelta, RunSnapshot } from "../observation/index.svelte.js";

export class StoryEventSource {
  static instances: StoryEventSource[] = [];
  readonly url: string;
  onopen: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private listeners = new Map<string, EventListener[]>();

  constructor(url: string | URL) {
    this.url = String(url);
    StoryEventSource.instances.push(this);
    setTimeout(() => this.onopen?.(new Event("open")), 0);
  }
  addEventListener(type: string, listener: EventListener) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }
  removeEventListener() {}
  close() {}
  send(type: "delta" | "snapshot", data: ObservationDelta | RunSnapshot) {
    const event = new MessageEvent(type, { data: JSON.stringify(data) });
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }
}

/** Replaces EventSource for the life of the Storybook preview. */
export function installStoryStream() {
  globalThis.EventSource = StoryEventSource as unknown as typeof EventSource;
}
