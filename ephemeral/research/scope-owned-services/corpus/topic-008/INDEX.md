# Extract transferable constraints

The comparison is limited to two approaches: Playwright Test `webServer` and
`start-server-and-test`. Both are bounded test workflows, not persistent
daemons or container-only managers. The local primary evidence is the
[Playwright documentation](sources/playwright-webserver.html), its
[web-server plugin](sources/playwright-webServerPlugin.ts#L72-L209), the
[`start-server-and-test` README](sources/start-server-and-test-README.md#L40-L65),
and its [lifecycle implementation](sources/start-server-and-test-index.js#L31-L155).

Downloaded-source provenance: [Playwright web-server docs](https://playwright.dev/docs/test-webserver), [Playwright plugin source](https://github.com/microsoft/playwright/blob/main/packages/playwright/src/plugins/webServerPlugin.ts), [`start-server-and-test` README](https://github.com/bahmutov/start-server-and-test/blob/master/README.md), and [`start-server-and-test` source](https://github.com/bahmutov/start-server-and-test/blob/master/src/index.js).

## Constraints to carry into a scope-owned Gimbal service

1. **Ownership spans the whole scope.** Start the command before owned work,
   retain it while that work runs, and always close it on normal completion,
   failure, or cancellation. The useful pattern is Playwright setup/teardown
   plus `start-server-and-test`'s `finally(stopServer)`, not detachment or
   reuse of an unrelated process.
2. **Required-service disappearance is failure.** Playwright races readiness
   against early exit and reports an error. `start-server-and-test` reports an
   unexpected close during startup, but removes that observer after readiness.
   Gimbal should preserve the stronger rule through the entire owning scope so
   a required service cannot disappear silently.
3. **Keep readiness ordinary and observable.** The tools show URL and output
   gates, but adopting their `wait-on`/regex frameworks would turn readiness
   into a new health-check contract. Gimbal should expose startup/runtime
   status and output so ordinary workflow checks can decide whether the service
   is usable.
4. **Shutdown needs a bounded group-level policy.** Playwright's documented
   graceful signal, wait, then forced process-group kill is the relevant model:
   signaling only the shell risks leaving descendants behind. The exact
   process-group/session mechanics remain platform work; the contract should
   require bounded cleanup and observable failure to clean up.
5. **Command evidence stays visible.** Inherited or reporter-routed stdout and
   stderr, plus the final exit/signal result, are necessary for diagnosing
   startup failure, unexpected exit, and shutdown. Silence or a boolean-only
   “running” result is insufficient.

## Mechanisms to reject as out of scope

- Playwright's `reuseExistingServer` and any persistent ownership: a service
  must belong to the declaring scope and not survive it.
- Playwright's array of web servers and `start-server-and-test`'s recursive
  multi-server ordering: these imply dependency graphs or orchestration rather
  than one required foreground service.
- Built-in URL/HTTP polling, stdout regex readiness, and generalized health
  checks: readiness belongs in ordinary workflow checks per issue 276.
- Automatic restart, persistent daemon registration, launchd integration, or
  Docker/Overmind-specific cleanup: the selected tools do not justify these,
  and the issue explicitly leaves the external tool's resources to its caller.

## Remaining evidence boundary

The sources establish lifecycle patterns, but they do not prove that
`tree-kill` and Playwright's process launcher have identical descendant
behavior on macOS and Linux. The implementation must keep that as an explicit
platform constraint rather than infer portability from these test tools.
