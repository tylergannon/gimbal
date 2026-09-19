import type { CommandRow, RunSnapshot, ScopeRow } from "../../observation/index.js";
import type { Graph } from "../../workflow/types.js";
import { at, total, usage } from "./support.js";

const run = "01M2QZ7PB3N6D0R9X2G5HKW8VY";
const workdir = "/Users/tyler/src/gimble";
const claude = "claude-fable-5-1";
const codex = "gpt-5.6-sol";

/** One ended scope of this record. Every scope here ended: a range-body
 * error in a PromiseLoop leaves the task and loop scopes ended with an empty
 * error while the run's own error carries the verdict. */
const ended = (
  key: string,
  name: string,
  began: string,
  close: string,
  loop = false,
): ScopeRow => ({
  run,
  key,
  name,
  loop,
  status: "ended",
  error: "",
  began: at(began),
  ended: at(close),
  values: {},
  decisions: [],
});

/** One recorded command. `id` is the scope's key with the command's own
 * `name.ordinal`, and the streams were kept under the run at that path. A
 * nonzero exit is an outcome, not a gimble error, so `error` stays empty
 * and `exit_code` is what the caller read. */
const command = (
  scope: string,
  name: string,
  line: string[],
  exit: number,
  started: string,
  close: string,
  stdout: string,
): CommandRow => {
  const id = `${scope}/${name}.1`;
  return {
    run,
    id,
    scope,
    name,
    command: line[0],
    args: line.slice(1),
    workdir,
    exit_code: exit,
    stdout,
    stderr: "",
    stdout_file: `commands/${id}/stdout.log`,
    stderr_file: `commands/${id}/stderr.log`,
    error: "",
    interrupted: false,
    started: at(started),
    ended: at(close),
    duration: at(close) - at(started),
  };
};

const passed = "ok  \tgithub.com/tylergannon/gimble/internal/observation\t0.402s";
/** The last four lines of the failing test, as the record kept them. */
const failed = [
  "--- FAIL: TestInterviewAnswer (0.02s)",
  "    rows_test.go:88: answered = 0, want > 0",
  "FAIL",
  "FAIL\tgimble/internal/observation\t0.412s",
].join("\n");

/** The failing command History.html selects: the fourth run of the `test`
 * call site, the one whose exit ended the run. */
export const recordedFailure: CommandRow = command(
  "implementation.1/task.4",
  "test",
  ["just", "test"],
  1,
  "2026-09-15T15:41:02",
  "2026-09-15T15:42:50",
  failed,
);

/** The record History.html reads: an implement-interview run that failed on
 * Sep 15 after four tasks. It is a record and nothing here is live. The
 * binary's implement-interview has changed since, so no compiled graph
 * matches it: the lanes stand on the rows alone. */
export const recordedRunSnapshot: RunSnapshot = {
  stream: "01M2QZ7QF5RJ8T2M6V0C4X9DWN",
  position: 2411,
  run: {
    id: run,
    name: "implement-interview",
    status: "failed",
    // The older workflow turned the command's exit into its own error and
    // returned it, so nothing after test.1 was observed.
    error: "implementation.1/task.4/test.1 exited 1",
    started: at("2026-09-15T14:31:02"),
    ended: at("2026-09-15T15:43:02"),
  },
  scopes: {
    // path.Base("") is ".", the name the runtime writes for the root scope.
    "": {
      ...ended("", ".", "2026-09-15T14:31:02", "2026-09-15T15:43:02"),
      values: {
        "requirements-file": { value: "/Users/tyler/src/gimble/tmp/issue-249.md" },
        "reference-directory": { value: "/Users/tyler/src/gimble/tmp/reference" },
        repository: { value: workdir },
      },
    },
    "reconnaissance.1": ended(
      "reconnaissance.1",
      "reconnaissance",
      "2026-09-15T14:31:04",
      "2026-09-15T14:44:10",
    ),
    "reconnaissance.1/backend.1": ended(
      "reconnaissance.1/backend.1",
      "backend",
      "2026-09-15T14:31:05",
      "2026-09-15T14:42:50",
    ),
    "reconnaissance.1/frontend.1": ended(
      "reconnaissance.1/frontend.1",
      "frontend",
      "2026-09-15T14:31:05",
      "2026-09-15T14:44:08",
    ),
    "implementation.1": ended(
      "implementation.1",
      "implementation",
      "2026-09-15T14:44:12",
      "2026-09-15T15:43:00",
      true,
    ),
    "implementation.1/task.1": {
      ...ended("implementation.1/task.1", "task", "2026-09-15T14:44:20", "2026-09-15T15:01:40"),
      task: { name: "Answer routing", validation: { command: "just test" } },
    },
    "implementation.1/task.2": {
      ...ended("implementation.1/task.2", "task", "2026-09-15T15:01:44", "2026-09-15T15:16:50"),
      task: { name: "Pending question list", validation: { command: "just test" } },
    },
    "implementation.1/task.3": {
      ...ended("implementation.1/task.3", "task", "2026-09-15T15:16:54", "2026-09-15T15:30:20"),
      task: { name: "Answered timestamp", validation: { command: "just test" } },
    },
    // Task 4 asked for no validation command, so it ran no task-check, and
    // its `test` is the one that ended the run.
    "implementation.1/task.4": {
      ...ended("implementation.1/task.4", "task", "2026-09-15T15:30:24", "2026-09-15T15:42:52"),
      task: { name: "Answered timestamp, second attempt", validation: { command: "" } },
    },
  },
  sessions: {
    "reconnaissance.1/backend.1/api-research.1": {
      run,
      id: "reconnaissance.1/backend.1/api-research.1",
      name: "api-research",
      adapter: "codex",
      model: codex,
      scope: "reconnaissance.1/backend.1",
      parent: "",
      created: at("2026-09-15T14:31:05"),
    },
    "reconnaissance.1/frontend.1/frontend-research.1": {
      run,
      id: "reconnaissance.1/frontend.1/frontend-research.1",
      name: "frontend-research",
      adapter: "codex",
      model: codex,
      scope: "reconnaissance.1/frontend.1",
      parent: "",
      created: at("2026-09-15T14:31:05"),
    },
    "sprint-planning.1": {
      run,
      id: "sprint-planning.1",
      name: "sprint-planning",
      adapter: "claude",
      model: claude,
      scope: "",
      parent: "",
      created: at("2026-09-15T14:44:11"),
    },
    "coding.1": {
      run,
      id: "coding.1",
      name: "coding",
      adapter: "claude",
      model: claude,
      scope: "",
      parent: "",
      created: at("2026-09-15T14:44:11"),
    },
    "implementation-scope-review.1": {
      run,
      id: "implementation-scope-review.1",
      name: "implementation-scope-review",
      adapter: "codex",
      model: codex,
      scope: "",
      parent: "",
      created: at("2026-09-15T14:44:11"),
    },
    "architectural-critique.1": {
      run,
      id: "architectural-critique.1",
      name: "architectural-critique",
      adapter: "codex",
      model: codex,
      scope: "",
      parent: "",
      created: at("2026-09-15T14:44:11"),
    },
    "implementation.1/task.1/qa-orchestration.1": {
      run,
      id: "implementation.1/task.1/qa-orchestration.1",
      name: "qa-orchestration",
      adapter: "claude",
      model: claude,
      scope: "implementation.1/task.1",
      parent: "",
      created: at("2026-09-15T14:56:30"),
    },
    "implementation.1/task.2/qa-orchestration.1": {
      run,
      id: "implementation.1/task.2/qa-orchestration.1",
      name: "qa-orchestration",
      adapter: "claude",
      model: claude,
      scope: "implementation.1/task.2",
      parent: "",
      created: at("2026-09-15T15:12:10"),
    },
    "implementation.1/task.3/qa-orchestration.1": {
      run,
      id: "implementation.1/task.3/qa-orchestration.1",
      name: "qa-orchestration",
      adapter: "claude",
      model: claude,
      scope: "implementation.1/task.3",
      parent: "",
      created: at("2026-09-15T15:26:02"),
    },
    // Task 4 never reached its validator: the run ended at test.1, so there
    // is no qa-orchestration session under it to observe.
  },
  interviews: {},
  turns: {
    "reconnaissance.1/backend.1/api-research.1/turn.1": {
      run,
      id: "reconnaissance.1/backend.1/api-research.1/turn.1",
      session: "reconnaissance.1/backend.1/api-research.1",
      scope: "reconnaissance.1/backend.1",
      prompt: "Read the saved issue, the Go API and the design docs. Do not edit source.",
      output_type: "gimble.Text",
      result: "backend/index.md records the observation rows and their provenance.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T14:31:06"),
      ended: at("2026-09-15T14:42:48"),
      duration: 702000,
    },
    "reconnaissance.1/frontend.1/frontend-research.1/turn.1": {
      run,
      id: "reconnaissance.1/frontend.1/frontend-research.1/turn.1",
      session: "reconnaissance.1/frontend.1/frontend-research.1",
      scope: "reconnaissance.1/frontend.1",
      prompt: "Read the current live-run UI and the installed shadcn-svelte components.",
      output_type: "gimble.Text",
      result: "frontend/index.md records the Svelte 5 runes and the components installed here.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T14:31:06"),
      ended: at("2026-09-15T14:44:06"),
      duration: 780000,
    },
    "coding.1/turn.1": {
      run,
      id: "coding.1/turn.1",
      session: "coding.1",
      scope: "implementation.1/task.1",
      prompt: "Implement only the selected task in the repository.",
      output_type: "gimble.Text",
      result: "Routed the accepted answer back to the waiting question.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T14:44:22"),
      ended: at("2026-09-15T14:52:30"),
      duration: 488000,
    },
    "coding.1/turn.2": {
      run,
      id: "coding.1/turn.2",
      session: "coding.1",
      scope: "implementation.1/task.2",
      prompt: "Implement only the selected task in the repository.",
      output_type: "gimble.Text",
      result: "Served every pending question with the scope it waits in.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T15:01:46"),
      ended: at("2026-09-15T15:09:20"),
      duration: 454000,
    },
    "coding.1/turn.3": {
      run,
      id: "coding.1/turn.3",
      session: "coding.1",
      scope: "implementation.1/task.3",
      prompt: "Implement only the selected task in the repository.",
      output_type: "gimble.Text",
      result: "Carried the answered timestamp through the answer flow.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T15:16:56"),
      ended: at("2026-09-15T15:24:40"),
      duration: 464000,
    },
    // 10m 04s, the last coding turn of the record.
    "coding.1/turn.4": {
      run,
      id: "coding.1/turn.4",
      session: "coding.1",
      scope: "implementation.1/task.4",
      prompt: "Implement only the selected task in the repository.",
      output_type: "gimble.Text",
      result: "Updated the answered row in place and rewrote its test.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T15:29:35"),
      ended: at("2026-09-15T15:39:39"),
      duration: 604000,
    },
    "implementation.1/task.1/qa-orchestration.1/turn.1": {
      run,
      id: "implementation.1/task.1/qa-orchestration.1/turn.1",
      session: "implementation.1/task.1/qa-orchestration.1",
      scope: "implementation.1/task.1",
      prompt: "Independently read the issue, the implementation and the recorded command results.",
      output_type: "implementinterview.Assessment",
      result: "The task-check for this task exited 1.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T14:56:30"),
      ended: at("2026-09-15T15:01:38"),
      duration: 308000,
    },
    "implementation.1/task.2/qa-orchestration.1/turn.1": {
      run,
      id: "implementation.1/task.2/qa-orchestration.1/turn.1",
      session: "implementation.1/task.2/qa-orchestration.1",
      scope: "implementation.1/task.2",
      prompt: "Independently read the issue, the implementation and the recorded command results.",
      output_type: "implementinterview.Assessment",
      result: "The answered row still carries no accepted time.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T15:12:10"),
      ended: at("2026-09-15T15:16:48"),
      duration: 278000,
    },
    "implementation.1/task.3/qa-orchestration.1/turn.1": {
      run,
      id: "implementation.1/task.3/qa-orchestration.1/turn.1",
      session: "implementation.1/task.3/qa-orchestration.1",
      scope: "implementation.1/task.3",
      prompt: "Independently read the issue, the implementation and the recorded command results.",
      output_type: "implementinterview.Assessment",
      result: "The page still needs a reload before the row changes.",
      error: "",
      interrupted: false,
      started: at("2026-09-15T15:26:02"),
      ended: at("2026-09-15T15:30:18"),
      duration: 256000,
    },
  },
  turn_usage: {
    "coding.1/turn.1": { [claude]: usage({ input: 48210, output: 6204, cache_read: 121400 }) },
    "coding.1/turn.2": { [claude]: usage({ input: 52108, output: 7180, cache_read: 168240 }) },
    "coding.1/turn.3": { [claude]: usage({ input: 56402, output: 8022, cache_read: 204610 }) },
    "coding.1/turn.4": { [claude]: usage({ input: 61044, output: 9106, cache_read: 241880 }) },
  },
  model_calls: {},
  commands: {
    "implementation.1/task.1/task-check.1": command(
      "implementation.1/task.1",
      "task-check",
      ["sh", "-lc", "just test"],
      1,
      "2026-09-15T14:52:32",
      "2026-09-15T14:53:20",
      failed,
    ),
    "implementation.1/task.1/build.1": command(
      "implementation.1/task.1",
      "build",
      ["just", "build"],
      0,
      "2026-09-15T14:53:21",
      "2026-09-15T14:54:02",
      "go build ./...",
    ),
    "implementation.1/task.1/vet.1": command(
      "implementation.1/task.1",
      "vet",
      ["just", "vet"],
      0,
      "2026-09-15T14:54:03",
      "2026-09-15T14:54:41",
      "go vet ./...",
    ),
    "implementation.1/task.1/test.1": command(
      "implementation.1/task.1",
      "test",
      ["just", "test"],
      0,
      "2026-09-15T14:54:42",
      "2026-09-15T14:56:28",
      passed,
    ),
    "implementation.1/task.2/task-check.1": command(
      "implementation.1/task.2",
      "task-check",
      ["sh", "-lc", "just test"],
      0,
      "2026-09-15T15:09:22",
      "2026-09-15T15:10:08",
      passed,
    ),
    "implementation.1/task.2/build.1": command(
      "implementation.1/task.2",
      "build",
      ["just", "build"],
      0,
      "2026-09-15T15:10:09",
      "2026-09-15T15:10:50",
      "go build ./...",
    ),
    "implementation.1/task.2/vet.1": command(
      "implementation.1/task.2",
      "vet",
      ["just", "vet"],
      0,
      "2026-09-15T15:10:51",
      "2026-09-15T15:11:29",
      "go vet ./...",
    ),
    "implementation.1/task.2/test.1": command(
      "implementation.1/task.2",
      "test",
      ["just", "test"],
      0,
      "2026-09-15T15:11:30",
      "2026-09-15T15:12:08",
      passed,
    ),
    "implementation.1/task.3/task-check.1": command(
      "implementation.1/task.3",
      "task-check",
      ["sh", "-lc", "just test"],
      0,
      "2026-09-15T15:24:42",
      "2026-09-15T15:25:28",
      passed,
    ),
    "implementation.1/task.3/build.1": command(
      "implementation.1/task.3",
      "build",
      ["just", "build"],
      0,
      "2026-09-15T15:25:29",
      "2026-09-15T15:26:10",
      "go build ./...",
    ),
    "implementation.1/task.3/vet.1": command(
      "implementation.1/task.3",
      "vet",
      ["just", "vet"],
      0,
      "2026-09-15T15:26:11",
      "2026-09-15T15:26:49",
      "go vet ./...",
    ),
    "implementation.1/task.3/test.1": command(
      "implementation.1/task.3",
      "test",
      ["just", "test"],
      0,
      "2026-09-15T15:26:50",
      "2026-09-15T15:28:36",
      passed,
    ),
    "implementation.1/task.4/build.1": command(
      "implementation.1/task.4",
      "build",
      ["just", "build"],
      0,
      "2026-09-15T15:39:40",
      "2026-09-15T15:40:21",
      "go build ./...",
    ),
    "implementation.1/task.4/vet.1": command(
      "implementation.1/task.4",
      "vet",
      ["just", "vet"],
      0,
      "2026-09-15T15:40:22",
      "2026-09-15T15:41:00",
      "go vet ./...",
    ),
    [recordedFailure.id]: recordedFailure,
  },
  // 412k in, 58k out, and no cost stated by any harness.
  totals: {
    scopes: {
      "": total(claude, { input: 412000, output: 58000, cache_read: 1042110 }),
      "reconnaissance.1": total(codex, { input: 51204, output: 4108 }),
      "reconnaissance.1/backend.1": total(codex, { input: 24102, output: 1906 }),
      "reconnaissance.1/frontend.1": total(codex, { input: 27102, output: 2202 }),
      "implementation.1": total(claude, { input: 360796, output: 53892, cache_read: 1042110 }),
      "implementation.1/task.1": total(claude, { input: 81420, output: 11206, cache_read: 121400 }),
      "implementation.1/task.2": total(claude, { input: 88304, output: 13188, cache_read: 168240 }),
      "implementation.1/task.3": total(claude, { input: 94028, output: 14392, cache_read: 204610 }),
      "implementation.1/task.4": total(claude, { input: 97044, output: 15106, cache_read: 241880 }),
    },
    sessions: {
      "coding.1": total(claude, { input: 217764, output: 30512, cache_read: 736130 }),
      "sprint-planning.1": total(claude, { input: 42108, output: 3204 }),
      "implementation-scope-review.1": total(codex, { input: 48210, output: 1402 }),
      "architectural-critique.1": total(codex, { input: 46802, output: 1318 }),
    },
  },
  transcripts: {},
};

/** History.html: a record with no graph. The binary's implement-interview
 * changed after this run, so the view is handed no `Graph` at all and draws
 * the lanes from the rows. */
export const recordedRun: { graph?: Graph; snapshot: RunSnapshot } = {
  snapshot: recordedRunSnapshot,
};
