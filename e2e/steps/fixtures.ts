import { type Page, type ElementHandle } from "@playwright/test";
import { createBdd, test as base } from "playwright-bdd";
import { appendFileSync, mkdirSync, readFileSync, readdirSync, rmSync, unlinkSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

export type BrowserState = {
  documents: number;
  expectedDocuments: number;
  pageErrors: string[];
  runElement?: ElementHandle<HTMLElement | SVGElement>;
  graphReady?: { revision: number; nodes: number; edges: number };
  graphElement?: ElementHandle<HTMLElement | SVGElement>;
};

export const test = base.extend<{ browserState: BrowserState }>({
  baseURL: async ({}, use) => {
    await use(process.env.BASE_URL || `http://127.0.0.1:${process.env.GIMBAL_E2E_PORT}`);
  },
  browserState: [
    async ({ page }, use) => {
      const state: BrowserState = { documents: 0, expectedDocuments: 1, pageErrors: [] };
      page.on("request", (request) => {
        if (request.resourceType() === "document") state.documents++;
      });
      page.on("pageerror", (error) => state.pageErrors.push(error.message));
      await use(state);
    },
    { auto: true },
  ],
});

const { Before, AfterStep } = createBdd(test);

const projectRuns = fileURLToPath(new URL("../fixtures/project/.gimbal/runs", import.meta.url));

Before(() => {
  if (!process.env.BASE_URL) resetRunsFixture();
});

AfterStep(async ({ page, $bddContext, $testInfo }) => {
  const step = $bddContext.bddTestData?.steps?.[$bddContext.stepIndex];
  if (step?.keywordType !== "Outcome") return;
  const directory = join(
    process.env.SKGO_E2E_ARTIFACTS ?? ".",
    "screenshots",
    process.env.SKGO_E2E_RUN ?? "run",
  );
  mkdirSync(directory, { recursive: true });
  const file = join(
    directory,
    `${slug($testInfo.title)}-${String($bddContext.stepIndex + 1).padStart(2, "0")}.png`,
  );
  await page.screenshot({ path: file, fullPage: true });
  await $testInfo.attach(step.textWithKeyword ?? $testInfo.title, {
    path: file,
    contentType: "image/png",
  });
});

export async function hydrated(page: Page): Promise<void> {
  await page.waitForFunction(() =>
    Object.keys(globalThis).some((key) => key.startsWith("__sveltekit_")),
  );
  await page.waitForFunction(() => history.scrollRestoration === "manual");
}

export function resetRunsFixture(): void {
  mkdirSync(projectRuns, { recursive: true });
  for (const entry of readdirSync(projectRuns)) {
    rmSync(join(projectRuns, entry), { recursive: true, force: true });
  }
}

export function publishFailedRunFixture(): void {
  const runDir = join(projectRuns, "fixture-failed");
  mkdirSync(runDir, { recursive: true });
  writeFileSync(join(runDir, "graph.json"), JSON.stringify({
    name: "implement", source: { file: "fixture.go", line: 1 }, services: [],
    body: [{ kind: "iterate", file: "fixture.go", line: 2, name: "outcome", services: [],
      body: [{ kind: "promise_loop", file: "fixture.go", line: 3, name: "implementation", planner: "implementation-planning", supervisors: [], services: [], body: [] }]
    }]
  }));
  const tables = {
    run: [
      {
        id: "fixture-failed",
        name: "implement",
        status: "failed",
        error: "FailureSummaryWithoutAnyBreaks".repeat(100),
        started: 1789800000000,
        ended: 1789800060000,
      },
    ],
    scopes: [
      {
        run: "fixture-failed",
        key: "",
        name: ".",
        loop: false,
        status: "ended",
        error: "fixture failed",
        began: 1789800000000,
        ended: 1789800060000,
        values: {},
        decisions: [],
      },
      {
        run: "fixture-failed",
        key: "outcome.1",
        name: "outcome.1",
        loop: false,
        status: "ended",
        error: "fixture failed",
        began: 1789800010000,
        ended: 1789800050000,
        values: {},
        decisions: [],
      },
      {
        run: "fixture-failed",
        key: "outcome.1/implementation.1",
        name: "implementation.1",
        loop: true,
        status: "ended",
        error: "fixture failed",
        began: 1789800010000,
        ended: 1789800050000,
        values: {},
        decisions: [],
      },
    ],
    sessions: [],
    turns: [],
    turn_usage: [],
    model_calls: [],
    commands: [],
    interactions: [],
  };
  for (const [table, rows] of Object.entries(tables)) {
    writeFileSync(join(runDir, `${table}.json`), JSON.stringify(rows));
  }
}

export function updateFailedRunSummary(summary: string): void {
  const runDir = join(projectRuns, "fixture-failed");
  const runFile = join(runDir, "run.json");
  const rows = JSON.parse(readFileSync(runFile, "utf8")) as Array<{ error: string }>;
  if (!rows[0]) throw new Error("fixture-failed has no run row");
  rows[0].error = summary;
  writeFileSync(runFile, JSON.stringify(rows));
  try {
    unlinkSync(join(runDir, "observation.json"));
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
}

export function publishInventoryFixture(): void {
  for (const [id, name, status, started, ended] of [
    ["foreign-pending", "plan-trip", "running", 1789800000000, 0],
    ["completed-release", "release", "completed", 1789799400000, 1789799850000],
  ] as const) {
    const dir = join(projectRuns, id);
    mkdirSync(dir, { recursive: true });
    const tables = {
      run: [{ id, name, status, error: "", started, ended }],
      scopes: [], sessions: [], turns: [], turn_usage: [], model_calls: [], commands: [],
      interactions: status === "running" ? [{
        run: id, question_id: "question-1", name: "preferences", scope: "research.1",
        session: "researcher.1", question: "Which cabin?", status: "pending",
        asked: 1789800001000, answered: 0, answer: "", messages: [],
      }] : [],
    };
    for (const [table, rows] of Object.entries(tables)) {
      writeFileSync(join(dir, `${table}.json`), JSON.stringify(rows));
    }
    if (status === "running") {
      // A live foreign writer publishes a checkpoint and contiguous durable
      // deltas. Bare table rewrites only model historical records.
      writeFileSync(join(dir, "observation.json"), JSON.stringify({
        snapshot: {
          stream: "inventory-fixture", position: 0, run: tables.run[0],
          interactions: { "question-1": tables.interactions[0] },
        },
        delta_offset: 0,
      }));
      writeFileSync(join(dir, "observation-deltas.jsonl"), "");
    }
  }
}

export function completeInventoryFixture(): void {
  const dir = join(projectRuns, "foreign-pending");
  const file = join(dir, "run.json");
  const rows = JSON.parse(readFileSync(file, "utf8"));
  Object.assign(rows[0], { status: "completed", ended: 1789800060000 });
  writeFileSync(file, JSON.stringify(rows));
  // Leave the pending interaction in the checkpoint: terminal runs must stop
  // asking for it. Publish completion through the real writer's journal seam.
  appendFileSync(join(dir, "observation-deltas.jsonl"), JSON.stringify({
    stream: "inventory-fixture", position: 1,
    frames: [{ type: "row", data: { table: "run", key: "foreign-pending", row: rows[0] } }],
  }) + "\n");
}

function slug(text: string): string {
  return text
    .replace(/[^\w\s.-]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .slice(0, 80);
}
