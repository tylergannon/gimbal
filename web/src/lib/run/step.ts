import Bot from "@lucide/svelte/icons/bot";
import Box from "@lucide/svelte/icons/box";
import GitFork from "@lucide/svelte/icons/git-fork";
import MessageCircleQuestionMark from "@lucide/svelte/icons/message-circle-question-mark";
import Repeat from "@lucide/svelte/icons/repeat";
import Terminal from "@lucide/svelte/icons/terminal";
import type { CommandRow, InterviewRow, ScopeRow, TurnRow } from "../observation/index.js";
import type { Graph } from "../workflow/types.js";
import type { RowStatus } from "./Pip.svelte";

/** One operation of a workflow body, exactly as the `Graph` writes it. */
type Operation = Graph["body"][number];

/** The three operations the map draws as a `Node`. A scope is not a node;
 * it is a `Sheet`. */
export type Step = Extract<Operation, { kind: "agent_call" | "command" | "interview" }>;

/** What a `Step` is, which is what picks its icon. */
export type StepKind = Step["kind"];

/** The three things a scope can be, which is what its sheet's label wears:
 * a plain scope, a `gimble.Group`, or a loop. */
export type ScopeKind = "scope" | "group" | "loop";

/** Everything the README's vocabulary names with an icon. */
export type MapKind = StepKind | ScopeKind;

/** A nested scope folded to one glyph: it is a step of its parent's body,
 * but the graph names it differently for each kind of scope, so the sheet
 * that draws it says which it is. */
export type ScopeStep = { kind: ScopeKind; name: string };

/** Every row a step of the map can have. A step the run has not reached has
 * no row at all, which is how a pip hears "not yet". */
export type StepRow = TurnRow | CommandRow | InterviewRow | ScopeRow;

/** The lucide icon `States.html` draws each one with. The icons live in one
 * place because a folded sheet shows the same glyph a `Node` shows. */
export const mapIcon: Record<MapKind, typeof Bot> = {
  agent_call: Bot,
  command: Terminal,
  interview: MessageCircleQuestionMark,
  scope: Box,
  group: GitFork,
  loop: Repeat,
};

/** What each one is called, for a reader who cannot see the icon. */
export const mapWording: Record<MapKind, string> = {
  agent_call: "agent call",
  command: "command",
  interview: "interview",
  scope: "scope",
  group: "group",
  loop: "loop",
};

/** The name the map shows for a step. An agent call is named by the session
 * that speaks, because two calls on one session are two steps of one
 * conversation; everything else carries its own name. */
export const stepName = (step: Step | ScopeStep): string =>
  "name" in step ? step.name : step.session;

/** How a row reads as a status and a failure.
 *
 * Only an interview and a scope carry a status of their own. A turn is
 * running until it has ended. A command's nonzero exit is an outcome rather
 * than an error, so the runtime leaves `error` empty and only `exit_code`
 * says the check failed. */
export const rowStatus = (row?: StepRow): { status?: RowStatus; error: string } => {
  if (row === undefined) return { error: "" };
  if ("question_id" in row) return { status: row.status, error: "" };
  if ("exit_code" in row) {
    const failure = row.error !== "" ? row.error : `exit status ${row.exit_code}`;
    return {
      status: row.ended === 0 ? "running" : "ended",
      error: row.ended !== 0 && (row.error !== "" || row.exit_code !== 0) ? failure : "",
    };
  }
  if ("key" in row) return { status: row.status, error: row.error };
  return { status: row.ended === 0 ? "running" : "ended", error: row.error };
};

/** The steps a body draws, in source order. Conditions are not drawn, so a
 * guarded step is a plain step of the body that holds it; whether it ran in
 * a given pass is what its row says. */
export const stepsOf = (body: Graph["body"]): Step[] =>
  body.flatMap((operation) => {
    if (operation.kind === "condition") {
      return operation.branches.flatMap((branch) => stepsOf(branch.body));
    }
    return operation.kind === "agent_call" ||
      operation.kind === "command" ||
      operation.kind === "interview"
      ? [operation]
      : [];
  });
