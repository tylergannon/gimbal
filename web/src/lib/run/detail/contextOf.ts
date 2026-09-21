// Splits "what was this agent told" into the bare call-site prompt and the
// scope values a workflow placed in context with gimble.Set(ctx, key,
// value). Pure functions only; no Svelte state lives here. See
// ephemeral/design/issue-325-detail-view-proposal.md section 2(d).
//
// A turn that recorded `context` has the workflow's own prompt in `prompt`.
// A turn saved before turns recorded their context has one glued string:
// prompt + "\n\n" + the rendered values.
import type {
  ContextEntry,
  RunSnapshot,
  ScopeValue,
  TurnRow,
} from "../../observation/index.svelte.js";

export type TurnContextEntry = ContextEntry;

/** One value visible from a scope: where it actually lives, what it hides,
 * and whether the scope shown is the one that set it. `complete` is only
 * meaningful for a row built from a turn's recorded context (whether the
 * full value was sent inline, or only an excerpt) — a row read straight off
 * a scope always has the whole value, so it stays undefined there. */
export type ContextRow = {
  key: string;
  scope: string;
  value: ScopeValue;
  shadows?: string[];
  here: boolean;
  complete?: boolean;
};

/** A turn's context rows, plus whether they came from the turn's own
 * recording or were inferred by walking the scope chain (a run saved before
 * turns recorded their context). */
export type TurnContext = { rows: ContextRow[]; recorded: boolean };

/** Outermost ancestor first, ending with `scopeKey` itself. Scope keys are
 * slash paths and the root scope's key is "". */
function ancestorChain(scopeKey: string): string[] {
  if (scopeKey === "") return [""];
  const chain = [""];
  let acc = "";
  for (const part of scopeKey.split("/")) {
    acc = acc === "" ? part : `${acc}/${part}`;
    chain.push(acc);
  }
  return chain;
}

/** Fold every entry recorded for one key — outermost first, nearest last —
 * into the winning (nearest) row; the rest become what it shadows. */
function fold(
  key: string,
  entries: { scope: string; value: ScopeValue; complete?: boolean }[],
  scopeKey: string,
): ContextRow {
  const winner = entries[entries.length - 1]!;
  const shadows = entries.slice(0, -1).map((entry) => entry.scope);
  return {
    key,
    scope: winner.scope,
    value: winner.value,
    shadows: shadows.length > 0 ? shadows : undefined,
    here: winner.scope === scopeKey,
    complete: winner.complete,
  };
}

/** Every value visible from `scopeKey`: its own values plus every
 * ancestor's, nearest scope winning per key. Rows are in the order the runtime
 * renders them: by owning scope, outermost first, and within a scope in the
 * order its keys were set. */
export function visibleContext(snapshot: RunSnapshot, scopeKey: string): ContextRow[] {
  const chain = ancestorChain(scopeKey);
  const order: string[] = [];
  const byKey = new Map<string, { scope: string; value: ScopeValue }[]>();
  for (const key of chain) {
    const scope = snapshot.scopes[key];
    if (!scope) continue;
    for (const [valueKey, value] of Object.entries(scope.values)) {
      let entries = byKey.get(valueKey);
      if (!entries) {
        entries = [];
        byKey.set(valueKey, entries);
        order.push(valueKey);
      }
      entries.push({ scope: key, value });
    }
  }
  const rows = order.map((key) => fold(key, byKey.get(key)!, scopeKey));
  // A stable sort keeps set order within one owning scope.
  return rows.sort((a, b) => chain.indexOf(a.scope) - chain.indexOf(b.scope));
}

/** A turn's context: the entries it recorded, joined with the value bodies on
 * the scope that owns each one. A turn saved before turns recorded their
 * context gets every value visible from the scope it ran in, flagged as
 * inferred. */
export function turnContext(snapshot: RunSnapshot, turn: TurnRow): TurnContext {
  if (turn.context === undefined) {
    return { rows: visibleContext(snapshot, turn.scope), recorded: false };
  }
  const chain = ancestorChain(turn.scope);
  const rows = turn.context.map((entry): ContextRow => {
    const outer = chain
      .slice(0, Math.max(chain.indexOf(entry.scope), 0))
      .filter((key) => snapshot.scopes[key]?.values[entry.key] !== undefined);
    return {
      key: entry.key,
      scope: entry.scope,
      value: snapshot.scopes[entry.scope]?.values[entry.key] ?? {},
      shadows: outer.length > 0 ? outer : undefined,
      here: entry.scope === turn.scope,
      complete: entry.complete,
    };
  });
  return { rows, recorded: true };
}

/** The prompt the workflow wrote, without the scope values sent after it.
 * A turn that recorded its context has exactly that in `prompt`. A turn saved
 * before then has the values glued on, starting at the section for the first
 * value visible from its scope; a "## " heading of the prompt's own is left
 * alone. */
export function barePrompt(turn: TurnRow, snapshot?: RunSnapshot): string {
  if (turn.context !== undefined || !snapshot) return turn.prompt;
  const first = visibleContext(snapshot, turn.scope)[0];
  if (!first) return turn.prompt;
  const boundary = turn.prompt.indexOf(`

## ${first.key}

`);
  return boundary === -1 ? turn.prompt : turn.prompt.slice(0, boundary);
}

/** A value's body as text: a string value as-is, any other JSON
 * pretty-printed, an artifact-backed value as its file path. */
export function valueText(value: ScopeValue): string {
  if (value.artifact) return value.artifact.file;
  if (value.value === undefined) return "";
  return typeof value.value === "string" ? value.value : JSON.stringify(value.value, null, 2);
}

/** A value's size in characters (an artifact reports its own byte size). */
export function valueSize(value: ScopeValue): number {
  if (value.artifact) return value.artifact.size;
  if (value.value === undefined) return 0;
  const text = typeof value.value === "string" ? value.value : JSON.stringify(value.value);
  return text.length;
}
