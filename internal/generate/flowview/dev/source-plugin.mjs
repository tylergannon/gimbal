import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import os from 'node:os';
import fs from 'node:fs/promises';

const execFileAsync = promisify(execFile);
const moduleRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const defaultWorkflows = [
  { name: 'validate-product', dir: './internal/workflows/validateproduct', entry: 'ValidateProduct' },
  { name: 'routing', dir: './internal/generate/testdata/inspection', entry: 'Routing' },
  { name: 'context-shape', dir: './internal/generate/testdata/inspection', entry: 'Inspect' },
];
const debounceMs = 250;

/** Provide source-only workflow projections to the standalone flowview dev app. */
export function workflowSource({ workflows = defaultWorkflows } = {}) {
  const states = new Map(workflows.map(({ name }) => [name, {
    revision: 0,
    status: 'updating',
    page: undefined,
    error: undefined,
  }]));
  let server;
  let tempDir;
  let generator;
  let closed = false;
  let activeController;
  let workerPromise;
  let cleanupPromise;
  let requested = 0;
  let lastChangeAt = 0;
  let wakeWorker;
  let refreshInputWatch = async () => {};
  let resolveInitial;
  const initial = new Promise(resolve => { resolveInitial = resolve; });
  let initialSettled = false;

  function settleInitial() {
    if (!initialSettled) {
      initialSettled = true;
      resolveInitial();
    }
  }

  function snapshot(name) {
    const state = states.get(name);
    return {
      ...(state.page === undefined ? {} : { page: state.page }),
      revision: state.revision,
      status: state.status,
      ...(state.error ? { error: state.error } : {}),
    };
  }

  function publish(name) {
    const data = { name, ...snapshot(name) };
    server?.ws.send({ type: 'custom', event: 'workflow:source', data });
  }

  function setState(name, status, page, error) {
    const state = states.get(name);
    state.revision++;
    state.status = status;
    if (page !== undefined) state.page = page;
    state.error = error;
    publish(name);
  }

  function markUpdating() {
    for (const { name } of workflows) setState(name, 'updating', undefined, undefined);
  }

  function requestRefresh() {
    requested++;
    lastChangeAt = Date.now();
    markUpdating();
    wakeWorker?.();
  }

  async function waitForChange(after) {
    if (closed || requested !== after) return;
    await new Promise(resolve => { wakeWorker = resolve; });
    wakeWorker = undefined;
  }

  async function waitForQuietPeriod() {
    while (!closed) {
      const remaining = debounceMs - (Date.now() - lastChangeAt);
      if (remaining <= 0) return;
      await new Promise(resolve => setTimeout(resolve, remaining));
    }
  }

  async function run(args, options = {}) {
    if (closed) throw Object.assign(new Error('source preview server is closing'), { code: 'ABORT_ERR' });
    const controller = new AbortController();
    activeController = controller;
    try {
      return await execFileAsync(args[0], args.slice(1), {
        cwd: moduleRoot,
        maxBuffer: 16 * 1024 * 1024,
        ...options,
        signal: controller.signal,
      });
    } finally {
      if (activeController === controller) activeController = undefined;
    }
  }

  function diagnostic(error) {
    const output = `${error?.stderr || ''}\n${error?.stdout || ''}`.trim();
    const detail = output.split(/\r?\n/).filter(Boolean).slice(-16).join('\n');
    const message = detail || error?.message || String(error);
    return message.length > 4000 ? `${message.slice(0, 3997)}...` : message;
  }

  async function buildGenerator() {
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'gimbal-flowview-'));
    generator = path.join(tempDir, process.platform === 'win32' ? 'gimbalgen.exe' : 'gimbalgen');
    await run(['go', 'build', '-o', generator, './internal/generate/gimbalgen']);
  }

  async function extract({ name, dir, entry }) {
    const output = path.join(tempDir, `${name}.json`);
    await run([generator, '-dir', dir, '-entry', entry, '-name', name, '-o', output]);
    return JSON.parse(await fs.readFile(output, 'utf8'));
  }

  async function extractLatest() {
    let processed = -1;
    while (!closed) {
      if (processed === requested) await waitForChange(processed);
      if (closed) return;
      if (processed >= 0) await waitForQuietPeriod();
      if (closed) return;

      const batch = requested;
      const results = [];
      for (const workflow of workflows) {
        if (closed) return;
        try {
          results.push({ name: workflow.name, page: await extract(workflow) });
        } catch (error) {
          if (closed) return;
          results.push({ name: workflow.name, error: diagnostic(error) });
        }
      }
      if (closed) return;

      // An edit arrived during extraction. Drop every result in this batch so
      // no older projection can overwrite the newest requested source.
      if (batch !== requested) {
        await waitForQuietPeriod();
        continue;
      }
      await refreshInputWatch();
      if (closed) return;
      if (batch !== requested) {
        await waitForQuietPeriod();
        continue;
      }
      for (const result of results) {
        if (result.error) setState(result.name, 'error', undefined, result.error);
        else setState(result.name, 'ready', result.page, undefined);
      }
      settleInitial();
      processed = batch;
    }
  }

  async function discoverInputs() {
    const files = new Set();
    const packageDirs = new Set();
    const embedDirs = new Set();
    const moduleRoots = new Set([moduleRoot]);

    try {
      const { stdout } = await run(['go', 'list', '-m', '-json', 'all']);
      for (const text of jsonObjects(stdout)) {
        let mod;
        try { mod = JSON.parse(text); } catch { continue; }
        if (mod.Main && mod.Dir) moduleRoots.add(path.resolve(mod.Dir));
        if (mod.Replace?.Dir) moduleRoots.add(path.resolve(mod.Replace.Dir));
      }
    } catch {}

    try {
      const { stdout } = await run(['go', 'list', '-deps', '-json', ...workflows.map(w => w.dir)]);
      for (const text of jsonObjects(stdout)) {
        let pkg;
        try { pkg = JSON.parse(text); } catch { continue; }
        if (!pkg.Dir || ![...moduleRoots].some(root => within(root, pkg.Dir))) continue;
        packageDirs.add(pkg.Dir);
        for (const name of [...(pkg.GoFiles || []), ...(pkg.CgoFiles || []), ...(pkg.CFiles || []), ...(pkg.CXXFiles || []), ...(pkg.MFiles || []), ...(pkg.HFiles || []), ...(pkg.SFiles || []), ...(pkg.SwigFiles || []), ...(pkg.SwigCXXFiles || [])]) {
          files.add(path.resolve(pkg.Dir, name));
        }
        for (const name of pkg.EmbedFiles || []) {
          const file = path.resolve(pkg.Dir, name);
          files.add(file);
          embedDirs.add(path.dirname(file));
        }
      }
    } catch {
      // The initial build/extraction will surface Go errors through the route;
      // known workflow packages are still watched as a useful recovery path.
      for (const workflow of workflows) packageDirs.add(path.resolve(moduleRoot, workflow.dir));
    }

    files.add(path.join(moduleRoot, 'go.mod'));
    files.add(path.join(moduleRoot, 'go.sum'));
    for (const root of moduleRoots) {
      files.add(path.join(root, 'go.mod'));
      files.add(path.join(root, 'go.sum'));
    }
    for (const file of ['go.work', 'go.work.sum']) files.add(path.join(moduleRoot, file));
    try {
      const { stdout } = await run(['go', 'env', 'GOWORK']);
      const workspace = stdout.trim();
      if (workspace && workspace !== 'off') {
        files.add(workspace);
        files.add(`${workspace}.sum`);
      }
    } catch {}
    const configFiles = new Set([...files].filter(file => ['go.mod', 'go.sum', 'go.work', 'go.work.sum'].includes(path.basename(file))));
    return { files, packageDirs, embedDirs, configFiles };
  }

  function within(root, candidate) {
    const relative = path.relative(root, candidate);
    return relative === '' || (!relative.startsWith(`..${path.sep}`) && relative !== '..' && !path.isAbsolute(relative));
  }

  function* jsonObjects(text) {
    let depth = 0;
    let start = -1;
    let quoted = false;
    let escaped = false;
    for (let i = 0; i < text.length; i++) {
      const c = text[i];
      if (quoted) {
        if (escaped) escaped = false;
        else if (c === '\\') escaped = true;
        else if (c === '"') quoted = false;
        continue;
      }
      if (c === '"') quoted = true;
      else if (c === '{') {
        if (depth++ === 0) start = i;
      } else if (c === '}' && --depth === 0 && start >= 0) {
        yield text.slice(start, i + 1);
        start = -1;
      }
    }
  }

  function isRelevantFile(file, inputs) {
    const absolute = path.resolve(file);
    if (inputs.configFiles.has(absolute)) return true;
    if (path.extname(absolute) === '.go' && inputs.packageDirs.has(path.dirname(absolute))) return true;
    return [...inputs.embedDirs].some(dir => within(dir, absolute));
  }

  function mergeInputs(previous, discovered) {
    return {
      files: new Set([...(previous?.files || []), ...discovered.files]),
      packageDirs: new Set([...(previous?.packageDirs || []), ...discovered.packageDirs]),
      embedDirs: new Set([...(previous?.embedDirs || []), ...discovered.embedDirs]),
      configFiles: new Set([...(previous?.configFiles || []), ...discovered.configFiles]),
    };
  }

  function cleanup() {
    if (cleanupPromise) return cleanupPromise;
    closed = true;
    wakeWorker?.();
    settleInitial();
    activeController?.abort();
    cleanupPromise = (async () => {
      await workerPromise?.catch(() => {});
      if (tempDir) await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {});
    })();
    return cleanupPromise;
  }

  return {
    name: 'gimbal-workflow-source',
    configureServer(viteServer) {
      server = viteServer;
      const middleware = async (req, res, next) => {
        if (req.method !== 'GET' || !req.url) return next();
        const pathname = new URL(req.url, 'http://vite.local').pathname;
        if (pathname === '/__workflow') {
          await initial;
          res.setHeader('content-type', 'application/json; charset=utf-8');
          res.setHeader('cache-control', 'no-store');
          res.end(JSON.stringify({ workflows: workflows.map(({ name }) => name) }));
          return;
        }
        const match = pathname.match(/^\/__workflow\/([^/]+)\/?$/);
        if (!match) return next();
        let name;
        try {
          name = decodeURIComponent(match[1]);
        } catch {
          res.statusCode = 400;
          res.setHeader('content-type', 'application/json; charset=utf-8');
          res.setHeader('cache-control', 'no-store');
          res.end(JSON.stringify({ error: 'malformed workflow name' }));
          return;
        }
        if (!states.has(name)) {
          res.statusCode = 404;
          res.setHeader('content-type', 'application/json; charset=utf-8');
          res.setHeader('cache-control', 'no-store');
          res.end(JSON.stringify({ error: `unknown workflow: ${name}` }));
          return;
        }
        await initial;
        res.setHeader('content-type', 'application/json; charset=utf-8');
        res.setHeader('cache-control', 'no-store');
        res.end(JSON.stringify(snapshot(name)));
      };
      viteServer.middlewares.use(middleware);

      let inputs;
      const watchedPaths = new Set();
      const watcher = (event, file) => {
        if (!closed && inputs && isRelevantFile(file, inputs)) requestRefresh();
      };
      const addInputsToWatcher = next => {
        const fresh = [...next.files, ...next.packageDirs, ...next.embedDirs].filter(file => !watchedPaths.has(file));
        for (const file of fresh) watchedPaths.add(file);
        if (fresh.length) viteServer.watcher.add(fresh);
      };
      const refreshInputs = async () => {
        try {
          inputs = mergeInputs(inputs, await discoverInputs());
          addInputsToWatcher(inputs);
        } catch {
          // Keep the previous closure when the edited source cannot be parsed.
        }
      };
      refreshInputWatch = refreshInputs;
      const start = async () => {
        try {
          inputs = mergeInputs(undefined, await discoverInputs());
          // Package directories contain direct Go inputs; embed roots watch
          // their full tree. This bounds watching to the local go-list closure.
          addInputsToWatcher(inputs);
          viteServer.watcher.on('all', watcher);
          await buildGenerator();
          await extractLatest();
        } catch (error) {
          if (!closed) {
            for (const { name } of workflows) setState(name, 'error', undefined, diagnostic(error));
            settleInitial();
          }
        }
      };
      workerPromise = start();
      viteServer.httpServer?.once('close', () => { void cleanup(); });
      viteServer.watcher.on('close', () => { void cleanup(); });
    },
    handleHotUpdate({ file }) {
      if (server && file && path.extname(file) === '.go') {
        // Go is source input to the extractor, not a browser module. The
        // watcher publishes workflow:source; suppress Vite's default update.
        return [];
      }
    },
    async closeBundle() {
      await cleanup();
    },
  };
}
