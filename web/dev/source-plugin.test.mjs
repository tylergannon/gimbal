import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "vite-plus";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { workflowSource } from "./source-plugin.mjs";

const moduleRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const flowviewRoot = path.join(moduleRoot, "web");

async function freePort() {
  const listener = net.createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const { port } = listener.address();
  await new Promise((resolve, reject) =>
    listener.close((error) => (error ? reject(error) : resolve())),
  );
  return port;
}

async function waitForSnapshot(url, predicate, timeout = 30000) {
  const end = Date.now() + timeout;
  let last;
  while (Date.now() < end) {
    const response = await fetch(url, { cache: "no-store" });
    assert.equal(response.status, 200);
    last = await response.json();
    if (predicate(last)) return last;
    await new Promise((resolve) => setTimeout(resolve, 80));
  }
  assert.fail(`snapshot did not reach expected state; last response: ${JSON.stringify(last)}`);
}

test("Vite source endpoint follows edits, errors, recovery, and newly imported local packages", async (t) => {
  const fixtureParent = path.join(moduleRoot, "internal/generate/testdata");
  await mkdir(fixtureParent, { recursive: true });
  const fixtureDir = await mkdtemp(path.join(fixtureParent, "flowview-fixture-"));
  let server;
  t.after(async () => {
    await server?.close();
    await rm(fixtureDir, { recursive: true, force: true });
  });

  const depDir = path.join(fixtureDir, "dep");
  await mkdir(depDir);
  const sourceFile = path.join(fixtureDir, "workflow.go");
  const depFile = path.join(depDir, "dep.go");
  const relativeDir = `./${path.relative(moduleRoot, fixtureDir).split(path.sep).join("/")}`;
  const importPath = `github.com/tylergannon/gimbal/${path.relative(moduleRoot, depDir).split(path.sep).join("/")}`;
  const writeWorkflow = async (marker, withDependency = true) =>
    writeFile(
      sourceFile,
      `package fixture

import (
  "context"
  ${withDependency ? `"${importPath}"` : ""}
  "github.com/tylergannon/gimbal"
)

func InspectFixture(ctx context.Context${withDependency ? ", shape dep.Shape" : ""}) error {
  gimbal.Set(ctx, "marker", "${marker}")
  ${withDependency ? "_ = shape.Name" : ""}
  return nil
}
`,
    );
  await writeFile(depFile, "package dep\ntype Shape struct { Name string }\n");
  await writeWorkflow("initial", false);

  server = await createServer({
    configFile: false,
    root: flowviewRoot,
    cacheDir: path.join(fixtureDir, ".vite-cache"),
    appType: "custom",
    plugins: [
      workflowSource({
        workflows: [{ name: "fixture", dir: relativeDir, entry: "InspectFixture" }],
      }),
    ],
    server: { host: "127.0.0.1", port: await freePort(), strictPort: true },
  });
  await server.listen();
  const address = server.httpServer.address();
  const endpoint = `http://127.0.0.1:${address.port}/@workflow-source/fixture`;

  const malformed = await fetch(`http://127.0.0.1:${address.port}/@workflow-source/%E0%A4%A`);
  assert.equal(malformed.status, 400);
  assert.equal(malformed.headers.get("cache-control"), "no-store");
  const initial = await waitForSnapshot(endpoint, (state) => state.status === "ready");
  assert.equal((await fetch(endpoint)).headers.get("cache-control"), "no-store");
  assert.match(JSON.stringify(initial.page), /initial/);

  await writeWorkflow("updated");
  const updated = await waitForSnapshot(
    endpoint,
    (state) =>
      state.status === "ready" &&
      state.revision > initial.revision &&
      JSON.stringify(state.page).includes("updated"),
  );
  assert.match(JSON.stringify(updated.page), /updated/);

  await new Promise((resolve) => setTimeout(resolve, 300));
  const beforeDependencyEdit = updated.revision;
  await writeFile(depFile, "package dep\ntype Shape struct { Name string; Added bool }\n");
  const dependencyUpdate = await waitForSnapshot(
    endpoint,
    (state) => state.status === "ready" && state.revision > beforeDependencyEdit,
  );
  assert.ok(dependencyUpdate.revision > beforeDependencyEdit);

  await writeFile(sourceFile, "package fixture\nfunc InspectFixture( {\n");
  const failed = await waitForSnapshot(
    endpoint,
    (state) => state.status === "error" && state.revision > dependencyUpdate.revision,
  );
  assert.match(JSON.stringify(failed.page), /updated/);
  assert.ok(failed.error);

  await writeFile(depFile, "package dep\ntype Shape struct { Name string; AddedAgain bool }\n");
  const dependencyWhileInvalid = await waitForSnapshot(
    endpoint,
    (state) => state.status === "error" && state.revision > failed.revision,
  );
  assert.match(JSON.stringify(dependencyWhileInvalid.page), /updated/);

  await writeWorkflow("recovered");
  const recovered = await waitForSnapshot(
    endpoint,
    (state) =>
      state.status === "ready" &&
      state.revision > dependencyWhileInvalid.revision &&
      JSON.stringify(state.page).includes("recovered"),
  );
  assert.match(JSON.stringify(recovered.page), /recovered/);
});
