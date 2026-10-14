import assert from "node:assert/strict";
import { once } from "node:events";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createServer } from "vite-plus";
import { appendFile, cp, lstat, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import test from "node:test";

const execFileAsync = promisify(execFile);

async function copyModule(source, target) {
  await cp(source, target, {
    recursive: true,
    filter: async (file) => {
      const info = await lstat(file);
      const name = path.basename(file);
      if (info.isDirectory())
        return !name.startsWith(".") && !["node_modules", "ephemeral", "bin"].includes(name);
      return (
        info.isFile() &&
        (/\.(go|json|txt|md|html)$/.test(name) ||
          ["go.mod", "go.sum", "source-plugin.mjs"].includes(name))
      );
    },
  });
}

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
  const temporary = await mkdtemp(path.join(os.tmpdir(), "gimbal-source-plugin-"));
  const sourceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
  const isolatedRoot = path.join(temporary, "module");
  let server;
  const environment = new Map();
  for (const [name, value] of Object.entries({
    GOWORK: "off",
    GIMBAL_GENERATE_CACHE_DIR: path.join(temporary, "gimbal-cache"),
    POLYTYPE_CACHE_DIR: path.join(temporary, "polytype-cache"),
    SKGO_CACHE_DIR: path.join(temporary, "skgo-cache"),
  })) {
    environment.set(name, process.env[name]);
    process.env[name] = value;
  }
  t.after(async () => {
    await server?.close();
    for (const [name, value] of environment) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
    await rm(temporary, { recursive: true, force: true });
  });
  await copyModule(sourceRoot, isolatedRoot);
  // Snapshot local replacements too: their go.mod files are watched inputs.
  const { stdout } = await execFileAsync("go", ["mod", "edit", "-json"], { cwd: isolatedRoot });
  for (const [index, replacement] of (JSON.parse(stdout).Replace || []).entries()) {
    if (replacement.New.Version) continue;
    const target = path.join(temporary, "replacements", String(index));
    await copyModule(path.resolve(sourceRoot, replacement.New.Path), target);
    const old =
      replacement.Old.Path + (replacement.Old.Version ? `@${replacement.Old.Version}` : "");
    await execFileAsync("go", ["mod", "edit", `-replace=${old}=${target}`], { cwd: isolatedRoot });
  }
  const { workflowSource } = await import(
    pathToFileURL(path.join(isolatedRoot, "web/dev/source-plugin.mjs"))
  );
  const moduleRoot = isolatedRoot;
  const flowviewRoot = path.join(moduleRoot, "web");
  const fixtureParent = path.join(moduleRoot, "internal/generate/testdata");
  await mkdir(fixtureParent, { recursive: true });
  const fixtureDir = await mkdtemp(path.join(fixtureParent, "flowview-fixture-"));

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
  const otherDir = path.join(fixtureDir, "other");
  await mkdir(otherDir);
  await writeFile(
    path.join(otherDir, "flow.go"),
    `package other
import("context";"github.com/tylergannon/gimbal")
func Other(ctx context.Context) {gimbal.Set(ctx,"independent","untouched")}
`,
  );
  const otherRelative = `./${path.relative(moduleRoot, otherDir).split(path.sep).join("/")}`;

  server = await createServer({
    configFile: false,
    root: flowviewRoot,
    cacheDir: path.join(fixtureDir, ".vite-cache"),
    appType: "custom",
    plugins: [
      workflowSource({
        workflows: [
          { name: "fixture", dir: relativeDir, entry: "InspectFixture" },
          { name: "other", dir: otherRelative, entry: "Other" },
        ],
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
  const otherEndpoint = `http://127.0.0.1:${address.port}/@workflow-source/other`;
  const otherInitial = await waitForSnapshot(otherEndpoint, (state) => state.status === "ready");
  assert.match(JSON.stringify(otherInitial.page), /untouched/);
  assert.equal(otherInitial.revision, 1);

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
  const otherAfter = await (await fetch(otherEndpoint)).json();
  assert.equal(otherAfter.status, "ready");
  assert.match(JSON.stringify(otherAfter.page), /untouched/);
  assert.equal(
    otherAfter.revision,
    1,
    "unaffected workflow must not be refreshed for another workflow's edits or failures",
  );
  await appendFile(path.join(moduleRoot, "go.mod"), "\n// fixture configuration edit\n");
  const afterConfig = await waitForSnapshot(
    otherEndpoint,
    (state) => state.status === "ready" && state.revision > 1,
  );
  assert.match(JSON.stringify(afterConfig.page), /untouched/);
});
