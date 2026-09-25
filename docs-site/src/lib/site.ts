// The one home for what the site says Gimbal is. The landing page, the header,
// the docs sidebar, and the link cards all read from here.

export const site = {
  name: "Gimbal",
  origin: "https://tylergannon.github.io",
  base: "https://tylergannon.github.io/gimbal",
  repo: "https://github.com/tylergannon/gimbal",
  category: "A Go runtime for multi-agent workflows",
  headline: "Agent workflows that read like pseudocode.",
  summary:
    "Write the workflow as a plain Go function. Gimbal gives it scoped context, planner loops, supervision, and a live console, on Codex, Claude Code, or your own harness.",
};

export const features = [
  {
    route: "/docs/console",
    name: "Live multi-agent console",
    line: "Watch every scope, turn, and command on one map. Steer an agent, answer its questions, or stop it.",
    shot: "shots/console.png",
  },
  {
    route: "/docs/workflows",
    name: "Workflows that read like pseudocode",
    line: "The whole process is one Go function: sessions, loops, branches, and prompts, all visible at the call site.",
    shot: "shots/planner.png",
  },
  {
    route: "/docs/primitives",
    name: "Primitives for agent work",
    line: "Scoped context, promise loops, supervision, and a graph read from your source.",
    shot: "shots/context.png",
  },
  {
    route: "/docs/roles",
    name: "Curated roles, bound to models",
    line: "Workflows name kinds of cognitive work. A run binds each role to a harness, model, and effort.",
    shot: "shots/supervision.png",
  },
  {
    route: "/docs/built-in",
    name: "Built-in workflows",
    line: "Implement, review, user-test, research, and summarize from the command line on day one.",
    shot: "shots/runs.png",
  },
  {
    route: "/docs/harnesses",
    name: "Any harness",
    line: "Codex, Claude Code, and Antigravity adapters ship in the box. Five methods add another.",
    shot: "shots/console.png",
  },
] as const;

export const primitives = [
  {
    id: "scope",
    code: "Scope",
    name: "Scopes",
    line: "A named lifetime. When it returns, its sessions close and its services stop.",
  },
  {
    id: "context",
    code: "Set · SetJSON",
    name: "Scoped context delivery",
    line: "Record a fact once. Every agent turn in that scope receives it, so prompts stay constants.",
  },
  {
    id: "promise-loop",
    code: "PromiseLoop",
    name: "Promise loop",
    line: "A planner keeps a backlog and picks the next task from the evidence so far. You range over it.",
  },
  {
    id: "supervision",
    code: "WithSupervisor",
    name: "Supervision",
    line: "A second agent watches a running turn and steers its objections in while the work is still going.",
  },
  {
    id: "graph",
    code: "go generate",
    name: "Statically mapped graph",
    line: "The workflow's shape is read from its source. The console draws the map before the first turn runs.",
  },
  {
    id: "concurrency",
    code: "Group · Iterate",
    name: "Observable concurrency",
    line: "errgroup-style branches and finite fan-out, each in its own scope on the map.",
  },
  {
    id: "evidence",
    code: "Check · Service",
    name: "Evidence and services",
    line: "Command results land in scope for the next agent to judge. A dev server lives exactly as long as its scope.",
  },
  {
    id: "interview",
    code: "Interview",
    name: "Interviews",
    line: "An agent asks a person questions. The run waits until the console answers.",
  },
] as const;

export const docs = [
  { route: "/docs/quickstart", title: "Quickstart" },
  { route: "/docs/console", title: "Console" },
  { route: "/docs/workflows", title: "Writing workflows" },
  { route: "/docs/primitives", title: "Primitives" },
  { route: "/docs/roles", title: "Workflow roles" },
  { route: "/docs/built-in", title: "Built-in workflows" },
  { route: "/docs/harnesses", title: "Harnesses" },
  { route: "/docs/about", title: "About" },
] as const;
