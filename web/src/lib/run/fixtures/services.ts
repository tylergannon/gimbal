import type { CommandRow, RunSnapshot, ScopeRow } from "../../observation/index.svelte.js";
import type { Graph } from "../../workflow/types.js";

const workflowFile = "examples/services/services.go";
const runID = "01M2SERVICES00000000000000";
const started = 1_800_000_000_000;

export const serviceOwnershipGraph: Graph = {
  name: "service-ownership",
  source: { file: workflowFile, line: 10 },
  services: [{ file: workflowFile, line: 11, name: "root-db" }],
  body: [
    { kind: "command", file: workflowFile, line: 12, name: "root-build" },
    {
      kind: "scope",
      file: workflowFile,
      line: 14,
      name: "backend",
      services: [{ file: workflowFile, line: 15, name: "api" }],
      body: [{ kind: "command", file: workflowFile, line: 16, name: "build" }],
    },
    {
      kind: "iterate",
      file: workflowFile,
      line: 20,
      name: "iteration",
      services: [{ file: workflowFile, line: 21, name: "fixture" }],
      body: [{ kind: "command", file: workflowFile, line: 22, name: "tests" }],
    },
  ],
  diagnostics: [],
};

function scope(key: string, name: string, began: number): ScopeRow {
  return {
    run: runID,
    key,
    name,
    loop: false,
    status: "running",
    error: "",
    began,
    ended: 0,
    values: {},
    decisions: [],
  };
}

function command(id: string, scopeKey: string, name: string, began: number): CommandRow {
  return {
    run: runID,
    id,
    scope: scopeKey,
    name,
    command: name,
    args: [],
    workdir: ".",
    exit_code: 0,
    stdout: "",
    stderr: "",
    stdout_file: "",
    stderr_file: "",
    error: "",
    interrupted: false,
    started: began,
    ended: began + 100,
    duration: 100,
  };
}

export const serviceOwnershipSnapshot: RunSnapshot = {
  stream: "fixture",
  position: 1,
  run: {
    id: runID,
    name: serviceOwnershipGraph.name,
    status: "running",
    error: "",
    started,
    ended: 0,
  },
  scopes: {
    "": scope("", serviceOwnershipGraph.name, started),
    "backend.1": scope("backend.1", "backend", started + 1_000),
    "iteration.1": scope("iteration.1", "iteration", started + 2_000),
    "iteration.2": scope("iteration.2", "iteration", started + 3_000),
  },
  sessions: {},
  interviews: {},
  turns: {},
  turn_usage: {},
  model_calls: {},
  commands: {
    "root-db.1": command("root-db.1", "", "root-db", started + 100),
    "root-build.1": command("root-build.1", "", "root-build", started + 200),
    "backend.1/api.1": command("backend.1/api.1", "backend.1", "api", started + 1_100),
    "backend.1/build.1": command("backend.1/build.1", "backend.1", "build", started + 1_200),
    "iteration.1/fixture.1": command(
      "iteration.1/fixture.1",
      "iteration.1",
      "fixture",
      started + 2_100,
    ),
    "iteration.1/tests.1": command("iteration.1/tests.1", "iteration.1", "tests", started + 2_200),
    "iteration.2/fixture.1": command(
      "iteration.2/fixture.1",
      "iteration.2",
      "fixture",
      started + 3_100,
    ),
    "iteration.2/tests.1": command("iteration.2/tests.1", "iteration.2", "tests", started + 3_200),
  },
  totals: { scopes: {}, sessions: {} },
  transcripts: {},
};

export const serviceOwnershipFixture = {
  graph: serviceOwnershipGraph,
  snapshot: serviceOwnershipSnapshot,
};
