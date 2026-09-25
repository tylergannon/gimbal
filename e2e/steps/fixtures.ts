import { type Page, type Route } from "@playwright/test";
import { createBdd, test as base } from "playwright-bdd";
import { mkdirSync, readFileSync, readdirSync, rmSync, unlinkSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

export type BrowserState = {
  documents: number;
  remoteMark: number;
  remotes: string[];
  pageErrors: string[];
  runsRequestSeen?: Promise<void>;
  releaseRunsRequest?: () => void;
  pendingNavigation?: Promise<void>;
};

export const test = base.extend<{ browserState: BrowserState }>({
  baseURL: async ({}, use) => {
    await use(process.env.BASE_URL || `http://127.0.0.1:${process.env.GIMBAL_E2E_PORT}`);
  },
  browserState: [
    async ({ page }, use) => {
      const state: BrowserState = { documents: 0, remoteMark: 0, remotes: [], pageErrors: [] };
      page.on("request", (request) => {
        if (request.resourceType() === "document") state.documents++;
        if (request.url().includes("/_app/remote/")) {
          state.remotes.push(`${request.method()} ${new URL(request.url()).pathname}`);
        }
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
  if (!process.env.BASE_URL) setRunsFixture(true);
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

export function setRunsFixture(populated: boolean): void {
  mkdirSync(projectRuns, { recursive: true });
  for (const entry of readdirSync(projectRuns)) {
    rmSync(join(projectRuns, entry), { recursive: true, force: true });
  }
  if (!populated) return;
  const runDir = join(projectRuns, "fixture-failed");
  mkdirSync(runDir, { recursive: true });
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
    interviews: [],
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

export function delayNextRunsData(page: Page, state: BrowserState): void {
  let markSeen!: () => void;
  let release!: () => void;
  state.runsRequestSeen = new Promise<void>((resolve) => (markSeen = resolve));
  const held = new Promise<void>((resolve) => (release = resolve));
  state.releaseRunsRequest = release;
  let delayed = false;
  void page.route("**/__data.json*", async (route: Route) => {
    if (!new URL(route.request().url()).pathname.endsWith("/__data.json") || delayed) {
      await route.continue();
      return;
    }
    delayed = true;
    markSeen();
    await held;
    await route.continue();
  });
}

function slug(text: string): string {
  return text
    .replace(/[^\w\s.-]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .slice(0, 80);
}
