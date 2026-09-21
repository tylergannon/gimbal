// Stand-ins for skgo remote functions in Storybook. The real modules
// (src/routes/*.remote.ts) are answered by the Go server; Storybook has no
// server, so .storybook/main.ts points every "*.remote" import at the files
// in ./mocks, which are built from these two helpers.
//
// A story decides what a remote answers:
//
//   import { setRemote } from "#lib/storybook/remotes.js";
//   setRemote("steer", async () => ({ landed: false }));
//
// A remote with no handler answers with its default and logs the call, so a
// page stays clickable without any setup.

type Handler = (arg: never) => unknown;

const handlers = new Map<string, Handler>();
const calls: { name: string; arg: unknown; at: number }[] = [];

/** Sets what the named remote answers until the next setRemote or resetRemotes. */
export function setRemote<Arg, Result>(
  name: string,
  handler: (arg: Arg) => Result | Promise<Result>,
) {
  handlers.set(name, handler as Handler);
}

export function resetRemotes() {
  handlers.clear();
  calls.length = 0;
}

/** Every remote call made so far, oldest first. */
export const remoteCalls = (): readonly { name: string; arg: unknown; at: number }[] => calls;

async function answer<Arg, Result>(name: string, arg: Arg, fallback: Result): Promise<Result> {
  calls.push({ name, arg, at: Date.now() });
  console.info(`[remote] ${name}`, arg);
  // Long enough to see a pending state, short enough not to be in the way.
  await new Promise((resolve) => setTimeout(resolve, 350));
  const handler = handlers.get(name) as ((arg: Arg) => Result | Promise<Result>) | undefined;
  return handler ? await handler(arg) : fallback;
}

export function mockCommand<Arg, Result>(name: string, fallback: Result) {
  return (arg: Arg) => answer(name, arg, fallback);
}

type Issue = { message: string; path: (string | number)[] };

function formInstance<Arg extends Record<string, unknown>, Result>(name: string, fallback: Result) {
  let values: Partial<Arg> = {};
  let issues: Issue[] = [];
  const field = (key: string) => ({
    value: () => values[key as keyof Arg],
    set: (value: unknown) => {
      values = { ...values, [key]: value };
    },
    issues: () => issues.filter((issue) => issue.path[0] === key),
    as: (type: string, value?: unknown) => ({
      name: key,
      type: type === "hidden" ? "hidden" : type,
      value: typeof value === "boolean" ? String(value) : (value ?? ""),
    }),
  });
  const fields = new Proxy(
    {
      set: (next: Partial<Arg>) => {
        values = { ...values, ...next };
      },
      value: () => values,
      allIssues: () => issues,
    } as Record<string, unknown>,
    { get: (target, key: string) => target[key] ?? field(key) },
  );
  const instance = {
    // What `{...form}` spreads onto a <form>: nothing may navigate away.
    method: "POST" as const,
    action: `?/remote=${name}`,
    onsubmit: (event: SubmitEvent) => event.preventDefault(),
    fields,
    result: undefined as Result | undefined,
    pending: 0,
    async submit() {
      instance.pending += 1;
      issues = [];
      try {
        instance.result = await answer(name, values as Arg, fallback);
        return true;
      } catch (error) {
        issues = [{ message: error instanceof Error ? error.message : String(error), path: [] }];
        instance.result = undefined;
        return false;
      } finally {
        instance.pending -= 1;
      }
    },
  };
  // method/action/onsubmit are the only enumerable keys, so a spread stays clean.
  for (const key of ["fields", "result", "pending", "submit"] as const)
    Object.defineProperty(instance, key, { enumerable: false, writable: true });
  return instance;
}

export function mockForm<Arg extends Record<string, unknown>, Result>(
  name: string,
  fallback: Result,
) {
  const keyed = new Map<string, ReturnType<typeof formInstance<Arg, Result>>>();
  const root = formInstance<Arg, Result>(name, fallback);
  return Object.assign(root, {
    for(key: string) {
      let instance = keyed.get(key);
      if (!instance) keyed.set(key, (instance = formInstance<Arg, Result>(name, fallback)));
      return instance;
    },
  });
}
