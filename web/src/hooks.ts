import type { RunSnapshot } from "#lib/observation/index.js";

// skgo renders pages in an embedded JavaScript engine, and that engine has no
// `structuredClone`: it is a platform global, not a language one, so nothing
// in the app's own bundle provides it. The observation state is plain JSON —
// it is what the run's JSON endpoint serves — so a JSON round trip is an
// exact clone of it, and the guard leaves the browser's real implementation
// alone. Without this, a page that clones its snapshot while rendering throws
// and the visitor gets the error page instead of the run.
if (typeof globalThis.structuredClone === "undefined") {
  globalThis.structuredClone = (<T>(value: T): T =>
    JSON.parse(JSON.stringify(value)) as T) as typeof structuredClone;
}

// The browser half of the `transport` entry src/hooks.go declares. Go encodes
// a run's observation as its own JSON, and this puts it back: what a load's
// `snapshot` property holds on both sides of the wire is the RunSnapshot
// itself, not the carrier it crossed in.
export const transport = {
  RunSnapshot: {
    // Nothing in this app sends a snapshot to the server, so there is no
    // value for the client to claim.
    encode: () => false as const,
    decode: ({ json }: { json: string }): RunSnapshot => JSON.parse(json) as RunSnapshot,
  },
};

// The generated `+page.server.ts` imports the transported type from here by
// the key's name, and what the page receives is the decoded snapshot.
export type { RunSnapshot };
