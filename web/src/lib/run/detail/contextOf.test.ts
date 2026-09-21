import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import type { RunSnapshot, ScopeRow, TurnRow } from "../../observation/index.svelte.js";
import { issue325Snapshot } from "../fixtures/issue-325.js";
import { barePrompt, turnContext, valueSize, valueText, visibleContext } from "./contextOf.js";

test("visibleContext walks the scope chain outermost first, nearest scope winning per key", () => {
  const rows = visibleContext(issue325Snapshot, "implementation.1/task.1");
  const byKey = new Map(rows.map((row) => [row.key, row]));

  // definition_of_done is set at both implementation.1 (the loop) and
  // implementation.1/task.1 (the task); the task's value wins and the loop's
  // is shadowed.
  const dod = byKey.get("definition_of_done");
  assert.ok(dod);
  assert.equal(dod?.scope, "implementation.1/task.1");
  assert.equal(dod?.here, true);
  assert.deepEqual(dod?.shadows, ["implementation.1"]);

  // repository_notes is only set at implementation.1, so it is inherited,
  // not "here", and shadows nothing.
  const notes = byKey.get("repository_notes");
  assert.ok(notes);
  assert.equal(notes?.scope, "implementation.1");
  assert.equal(notes?.here, false);
  assert.equal(notes?.shadows, undefined);

  // assignment is only set at the task scope itself.
  const assignment = byKey.get("assignment");
  assert.ok(assignment);
  assert.equal(assignment?.scope, "implementation.1/task.1");
  assert.equal(assignment?.here, true);
});

test("visibleContext at the loop scope sees only its own values, none from the task below it", () => {
  const rows = visibleContext(issue325Snapshot, "implementation.1");
  const keys = rows.map((row) => row.key).sort();
  assert.deepEqual(keys, ["definition_of_done", "repository_notes"]);
  assert.ok(rows.every((row) => row.here));
});

test("visibleContext at the root scope, or an unknown scope, returns no rows", () => {
  assert.deepEqual(visibleContext(issue325Snapshot, ""), []);
  assert.deepEqual(visibleContext(issue325Snapshot, "no/such/scope"), []);
});

test("turnContext uses recorded entries, joining bodies from the owning scope and preferring the nearer one", () => {
  const turn = issue325Snapshot.turns["coding.1/turn.1"];
  assert.ok(turn.context && turn.context.length > 0);
  const { rows, recorded } = turnContext(issue325Snapshot, turn);
  assert.equal(recorded, true);

  const byKey = new Map(rows.map((row) => [row.key, row]));
  assert.deepEqual(
    rows.map((row) => row.key),
    turn.context.map((entry) => entry.key),
    "rows keep the recorded order",
  );

  const dod = byKey.get("definition_of_done");
  assert.ok(dod);
  // The task scope owns the key; the loop's value for it is what it shadows.
  assert.equal(dod?.scope, "implementation.1/task.1");
  assert.deepEqual(dod?.shadows, ["implementation.1"]);
  assert.equal(dod?.complete, true);

  const notes = byKey.get("repository_notes");
  assert.ok(notes);
  assert.equal(notes?.complete, false);
  assert.ok(notes?.value.artifact, "repository_notes is artifact-backed");
});

test("turnContext falls back to visibleContext, flagged inferred, when a turn recorded no context", () => {
  const turn: TurnRow = {
    ...(issue325Snapshot.turns["coding.1/turn.1"] as TurnRow),
    context: undefined,
  };
  const { rows, recorded } = turnContext(issue325Snapshot, turn);
  assert.equal(recorded, false);
  assert.deepEqual(
    rows.map((row) => row.key).sort(),
    visibleContext(issue325Snapshot, turn.scope)
      .map((row) => row.key)
      .sort(),
  );
});

test("barePrompt is the recorded prompt when the turn recorded its context", () => {
  const turn = issue325Snapshot.turns["coding.1/turn.1"];
  assert.equal(barePrompt(turn, issue325Snapshot), turn.prompt);
});

test("barePrompt leaves a heading of the prompt's own alone", () => {
  const turn: TurnRow = {
    ...issue325Snapshot.turns["coding.1/turn.1"],
    context: undefined,
    prompt: "Do the thing.\n\n## Rules\n\nBe brief.",
  };
  assert.equal(barePrompt(turn, issue325Snapshot), turn.prompt);
});

test("barePrompt falls back to the text before the first section boundary in an old glued prompt", () => {
  const turn: TurnRow = {
    run: "r",
    id: "t",
    session: "s",
    scope: "implementation.1/task.1",
    prompt: "Do the thing.\n\n## repository_notes\n\nNotes.\n\n## assignment\n\nBuild it.",
    output_type: "text",
    result: "",
    error: "",
    interrupted: false,
    started: 0,
    ended: 0,
    duration: 0,
  };
  assert.equal(barePrompt(turn, issue325Snapshot), "Do the thing.");
});

test("barePrompt returns the whole prompt when there is no section boundary at all", () => {
  const turn: TurnRow = {
    run: "r",
    id: "t",
    session: "s",
    scope: "",
    prompt: "Just a plain instruction with no folded sections.",
    output_type: "text",
    result: "",
    error: "",
    interrupted: false,
    started: 0,
    ended: 0,
    duration: 0,
  };
  assert.equal(barePrompt(turn), turn.prompt);
});

test("valueText renders a string as-is, other JSON pretty-printed, and an artifact as its path", () => {
  assert.equal(valueText({ value: "hello" }), "hello");
  assert.equal(valueText({ value: { a: 1 } }), JSON.stringify({ a: 1 }, null, 2));
  assert.equal(
    valueText({ artifact: { file: "artifacts/notes.md", size: 10, format: "text", preview: "…" } }),
    "artifacts/notes.md",
  );
  assert.equal(valueText({}), "");
});

test("valueSize counts characters for an inline value and reports the artifact's own size", () => {
  assert.equal(valueSize({ value: "hello" }), 5);
  assert.equal(
    valueSize({
      artifact: { file: "artifacts/notes.md", size: 4096, format: "text", preview: "…" },
    }),
    4096,
  );
  assert.equal(valueSize({}), 0);
});

test("a value shadowed twice over (root, mid, leaf all set the same key) still folds to one row", () => {
  const snapshot: RunSnapshot = {
    stream: "s",
    position: 0,
    run: { id: "r", name: "r", status: "running", error: "", started: 0, ended: 0 },
    scopes: {
      "": rowFor("", { note: { value: "root note" } }),
      a: rowFor("a", { note: { value: "a note" } }),
      "a/b": rowFor("a/b", { note: { value: "b note" } }),
    },
    sessions: {},
    interviews: {},
    turns: {},
    turn_usage: {},
    model_calls: {},
    commands: {},
    totals: { scopes: {}, sessions: {} },
    transcripts: {},
  };
  const rows = visibleContext(snapshot, "a/b");
  assert.equal(rows.length, 1);
  assert.equal(rows[0]?.scope, "a/b");
  assert.deepEqual(rows[0]?.shadows, ["", "a"]);
});

function rowFor(key: string, values: ScopeRow["values"]): ScopeRow {
  return {
    run: "r",
    key,
    name: key || "root",
    loop: false,
    status: "running",
    error: "",
    began: 0,
    ended: 0,
    values,
    decisions: [],
  };
}
