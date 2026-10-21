import { spawn } from "node:child_process";
import { chromium } from "playwright";
import { playwright, PlaywrightBrowserProvider } from "vite-plus/test/browser-playwright";

const lightpanda = playwright();
delete lightpanda.prewarm;

lightpanda.providerFactory = (project) => {
  const provider = new PlaywrightBrowserProvider(project, { actionTimeout: 5_000 });
  provider.supportsParallelism = false;
  let starting: Promise<void> | undefined;
  let stop = async () => {};

  async function start() {
    const child = spawn(
      process.env.LIGHTPANDA_BINARY ?? "lightpanda",
      [
        "serve",
        "--host",
        "127.0.0.1",
        "--port",
        "0",
        "--load-resources",
        "iframe",
        "--load-resources",
        "stylesheet",
        "--load-resources",
        "worker",
      ],
      { stdio: ["ignore", "ignore", "pipe"] },
    );
    const exited = new Promise<void>((resolve) => child.once("exit", () => resolve()));
    stop = async () => {
      if (child.pid && child.exitCode === null && child.signalCode === null) {
        child.kill();
        await exited;
      }
    };

    try {
      const endpoint = await new Promise<string>((resolve, reject) => {
        let diagnostic = "";
        const timeout = setTimeout(
          () => reject(new Error(`Lightpanda did not start: ${diagnostic}`)),
          10_000,
        );
        const finish = (error?: Error, url?: string) => {
          clearTimeout(timeout);
          if (error) reject(error);
          else resolve(url!);
        };
        child.once("error", (error) => finish(error));
        child.once("exit", (code) =>
          finish(new Error(`Lightpanda exited (${code}): ${diagnostic}`)),
        );
        child.stderr.on("data", (chunk: Buffer) => {
          diagnostic = (diagnostic + chunk.toString()).slice(-4000);
          const match = diagnostic.match(/address=127\.0\.0\.1:(\d+)/);
          if (match) finish(undefined, `ws://127.0.0.1:${match[1]}`);
        });
      });
      provider.browser = await chromium.connectOverCDP(endpoint);
    } catch (error) {
      await stop();
      throw error;
    }
  }

  // Static ESM linking blocks Lightpanda's WebSocket callbacks. Resolve
  // factories while registration can still exchange messages with Vitest.
  const register = provider.mocker.register.bind(provider.mocker);
  provider.mocker.register = async (id, module) => {
    if (module.type === "manual") await module.resolve();
    return register(id, module);
  };
  const openPage = provider.openPage.bind(provider);
  provider.openPage = async (...args) => {
    await (starting ??= start());
    return openPage(...args);
  };
  const close = provider.close.bind(provider);
  provider.close = async () => {
    try {
      await starting?.catch(() => {});
      await close();
    } finally {
      await stop();
    }
  };
  return provider;
};

export default lightpanda;
