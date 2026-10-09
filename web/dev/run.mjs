// One owner for the frontend and Go application during workflow development.
import { spawn } from "node:child_process";
import { access, mkdtemp, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import os from "node:os";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const port = process.env.GIMBAL_DEV_PORT || "8080";
const vitePort = process.env.GIMBAL_DEV_VITE_PORT || "5173";
const origin = `http://127.0.0.1:${port}`;
const children = new Set();
let stopping = false;
let temp;
function launch(command, args, env = {}) {
  const child = spawn(command, args, {
    cwd: root,
    env: { ...process.env, ...env },
    stdio: "inherit",
  });
  children.add(child);
  child.once("exit", () => children.delete(child));
  return child;
}
function done(child) {
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) =>
      code === 0 ? resolve() : reject(new Error(`Process ended with ${code ?? signal}`)),
    );
  });
}
async function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  const active = [...children];
  for (const child of active) child.kill("SIGTERM");
  const timer = setTimeout(() => {
    for (const child of children) child.kill("SIGKILL");
  }, 4000);
  timer.unref();
  await Promise.all(
    active.map((child) =>
      child.exitCode !== null
        ? Promise.resolve()
        : new Promise((resolve) => child.once("exit", resolve)),
    ),
  );
  clearTimeout(timer);
  if (temp) await rm(temp, { recursive: true, force: true });
  process.exitCode = code;
}
process.once("SIGINT", () => {
  void stop();
});
process.once("SIGTERM", () => {
  void stop();
});
try {
  // A fresh checkout needs the generated manifest and embedded application.
  try {
    await access(path.join(root, "web/build/skgo.manifest.json"));
  } catch {
    await done(launch("just", ["build"]));
  }
  if (!stopping) {
    await done(launch("go", ["generate", "./internal/skgo"]));
    temp = await mkdtemp(path.join(os.tmpdir(), "gimbal-dev-"));
    const binary = path.join(temp, "gimbal");
    await done(launch("go", ["build", "-o", binary, "./cmd/gimbal"]));
    const frontend = launch(
      "pnpm",
      [
        "--dir",
        "web",
        "exec",
        "vp",
        "dev",
        "--host",
        "127.0.0.1",
        "--port",
        vitePort,
        "--strictPort",
      ],
      { ORIGIN: origin },
    );
    for (let attempts = 0; attempts < 100 && !stopping; attempts++) {
      if (frontend.exitCode !== null) throw new Error("Frontend did not start");
      try {
        const response = await fetch(`http://127.0.0.1:${vitePort}/@vite/client`);
        if (response.ok) break;
      } catch {}
      if (attempts === 99) throw new Error("Frontend startup timed out");
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    if (!stopping) {
      const backend = launch(binary, ["--port", port], {
        GIMBAL_WEB_PROXY: `http://127.0.0.1:${vitePort}`,
        GIMBAL_WEB_ORIGIN: origin,
      });
      console.log(`Workflow development: ${origin}/workflows`);
      await Promise.race([done(frontend), done(backend)]);
      await stop();
    }
  }
} catch (error) {
  console.error(error.message);
  await stop(1);
}
