// Lightpanda cannot service Vitest's lazy mock RPC during synchronous ESM
// linking. Resolve factories after registration, then prime the server's
// export keys before importing their consumers. Keep importOriginal outside
// the registration queue so it cannot wait for itself.
const mocker = globalThis.__vitest_mocker__;
const runner = globalThis.__vitest_browser_runner__;
const order = [];
const warmed = new WeakSet();
const queueMock = mocker.queueMock.bind(mocker);
mocker.queueMock = (...args) => {
  order.push(args[0]);
  return queueMock(...args);
};
let resolving = false;
const wrap = runner.wrapModule;
async function resolveThenImport(factory) {
  await mocker.prepare();
  if (!resolving) {
    resolving = true;
    try {
      const mocks = Array.from(mocker.registry.keys(), (key) => mocker.registry.get(key));
      mocks.sort((a, b) => order.indexOf(a.raw) - order.indexOf(b.raw));
      for (const mock of mocks) {
        if (mock.type === "manual" && !warmed.has(mock)) {
          await mock.resolve();
          await mocker.interceptor.register(mock);
          warmed.add(mock);
        }
      }
    } finally {
      resolving = false;
    }
  }
  return wrap(factory);
}
runner.wrapModule = resolveThenImport;
runner.wrapDynamicImport = resolveThenImport;
