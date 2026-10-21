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
        "--http-timeout",
        "0",
        "--log-format",
        "logfmt",
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

  // The browser setup registers a resolved factory again. Only that second
  // registration can safely ask the browser for its already-cached exports.
  const registered = new Set<string>();
  const register = provider.mocker.register.bind(provider.mocker);
  provider.mocker.register = async (id, module) => {
    const key = `${id}:${module.url}`;
    if (module.type === "manual" && registered.has(key)) await module.resolve();
    registered.add(key);
    return register(id, module);
  };
  const clear = provider.mocker.clear.bind(provider.mocker);
  provider.mocker.clear = async (id) => {
    for (const key of registered) if (key.startsWith(`${id}:`)) registered.delete(key);
    return clear(id);
  };
  // Lightpanda's CDP frame metadata currently omits the iframe's name.
  const commands = provider.getCommandsContext.bind(provider);
  provider.getCommandsContext = (id) => {
    const context = commands(id);
    return {
      ...context,
      frame: async () => {
        const handle = await context.page.locator("iframe[data-vitest]").elementHandle();
        const frame = await handle?.contentFrame();
        await handle?.dispose();
        if (!frame) throw new Error("Vitest test iframe is unavailable");
        return frame;
      },
    };
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
