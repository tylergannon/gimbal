<script lang="ts">
  import { asset, resolve } from "$app/paths";
  import InstallPrompt from "#lib/components/InstallPrompt.svelte";
  import Shot from "#lib/components/Shot.svelte";
  import AdapterCode from "#lib/snippets/AdapterCode.svx";
  import LoopCode from "#lib/snippets/LoopCode.svx";
  import RunHelp from "#lib/snippets/RunHelp.svx";
  import { features, primitives, site } from "#lib/site";
  import defaults from "../../../internal/builtin/defaults.json";

  const title = `${site.name} — ${site.headline}`;
  const canonical = `${site.base}/`;
  const bindings = Object.entries(defaults as Record<string, string>);
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={site.summary} />
  <link rel="canonical" href={canonical} />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={site.summary} />
  <meta property="og:url" content={canonical} />
  <meta property="og:image" content={`${site.base}/og.png`} />
  <meta name="twitter:card" content="summary_large_image" />
</svelte:head>

<main class="overflow-x-clip">
  <!-- The poster: what it is, what is in the box, how to get it. -->
  <section class="shell grid grid-cols-1 gap-8 pt-8 pb-8 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.25fr)] lg:items-center lg:gap-12 lg:pt-9">
    <div>
      <p class="eyebrow mb-4">{site.category}</p>
      <h1 class="font-serif text-[clamp(2.4rem,4vw,3.5rem)] leading-[1.02] font-medium tracking-[-0.035em] text-balance">
        Agent workflows that read like <em class="text-primary font-normal">pseudocode.</em>
      </h1>
      <p class="text-muted-foreground mt-4 max-w-xl text-base leading-7">{site.summary}</p>

      <ul class="border-border mt-5 grid border-t sm:grid-cols-2">
        {#each features as feature, i}
          <li class="border-border border-b sm:odd:border-r">
            <a class="group hover:bg-muted/40 flex items-baseline gap-3 px-1 py-2 text-sm transition-colors sm:px-3" href={resolve(feature.route)}>
              <span class="text-primary font-mono text-[0.65rem]">{String(i + 1).padStart(2, "0")}</span>
              <span class="font-medium">{feature.name}</span>
            </a>
          </li>
        {/each}
      </ul>

      <div class="mt-5 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div class="min-w-0 flex-1"><InstallPrompt /></div>
        <a class="bg-primary text-primary-foreground inline-flex h-12 shrink-0 items-center justify-center rounded-lg px-5 text-sm font-semibold transition-opacity hover:opacity-90" href={resolve("/docs/quickstart")}>Quickstart →</a>
      </div>
    </div>

    <div class="min-w-0">
      <figure class="frame overflow-hidden">
        <div class="frame-bar"><i></i><i></i><i></i><span>Gimble · real evaluator run</span></div>
        <video controls playsinline preload="metadata" poster={asset("shots/evaluator-poster.png")} class="block aspect-video w-full bg-black" aria-label="An evaluator using Gimble to implement and test game persistence">
          <source src={asset("videos/validate-product-demo.mp4")} type="video/mp4" />
          Your browser does not support embedded video.
        </video>
        <figcaption class="text-muted-foreground px-3 py-2 text-xs">A real agent run, edited to 2.5× speed for a quick look.</figcaption>
      </figure>
    </div>
  </section>

  <!-- The primitives, one glance each. -->
  <section class="border-border border-y bg-[oklch(0.155_0.012_55)]">
    <div class="shell py-7">
      <div class="mb-4 flex flex-wrap items-baseline justify-between gap-3">
        <h2 class="eyebrow">The primitives</h2>
        <a class="text-muted-foreground hover:text-foreground text-sm transition-colors" href={resolve("/docs/primitives")}>All primitives →</a>
      </div>
      <div class="bg-border grid gap-px overflow-hidden rounded-xl sm:grid-cols-2 lg:grid-cols-4">
        {#each primitives as primitive}
          <a class="bg-background hover:bg-card block p-4 transition-colors" href={`${resolve("/docs/primitives")}#${primitive.id}`}>
            <code class="text-primary text-xs">{primitive.code}</code>
            <h3 class="mt-1.5 font-semibold">{primitive.name}</h3>
            <p class="text-muted-foreground mt-1 text-sm leading-6">{primitive.line}</p>
          </a>
        {/each}
      </div>
    </div>
  </section>

  <!-- Code beside its run. -->
  <section class="shell py-20">
    <p class="eyebrow mb-4">Workflows that read like pseudocode</p>
    <h2 class="max-w-3xl font-serif text-4xl leading-[1.05] font-medium tracking-[-0.03em] sm:text-5xl">The code on the left is the map on the right.</h2>
    <p class="text-muted-foreground mt-5 max-w-2xl text-lg leading-8">A planner chooses tasks, a coder does each one under a coach's eye, a command gathers evidence, and an independent validator decides. No DSL, no YAML, no hidden retry policy. It is a <code class="text-foreground">for</code> loop.</p>

    <div class="mt-10 grid gap-6 lg:grid-cols-2 lg:items-start">
      <div class="frame code">
        <div class="frame-bar"><i></i><i></i><i></i><span>implementation.go · condensed</span></div>
        <LoopCode />
      </div>
      <Shot src="shots/planner.png" alt="The current console map of a planner loop, with the planner highlighted above a task scope, coding turn, watcher, check, and validator." label="a recorded implementation run" crop />
    </div>
    <a class="text-primary mt-6 inline-block text-sm font-medium" href={resolve("/docs/workflows")}>How workflows are written →</a>
  </section>

  <!-- The console. -->
  <section class="border-border border-t">
    <div class="shell grid gap-10 py-20 lg:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)] lg:items-center">
      <div>
        <p class="eyebrow mb-4">Live multi-agent console</p>
        <h2 class="font-serif text-4xl leading-[1.05] font-medium tracking-[-0.03em]">See the run. Reach into it.</h2>
        <ul class="text-muted-foreground mt-6 space-y-3 leading-7">
          <li><strong class="text-foreground">One map</strong> of every scope, turn, command, and watcher, live as it happens.</li>
          <li><strong class="text-foreground">Select anything</strong> to read its assignment, the context it was sent, its model calls, and its transcript.</li>
          <li><strong class="text-foreground">Steer</strong> a running agent, <strong class="text-foreground">answer</strong> its interview questions, stop a turn, or cancel the run.</li>
          <li><strong class="text-foreground">Every run is kept.</strong> Cost, elapsed time, sessions, and turns for each one.</li>
        </ul>
        <a class="text-primary mt-6 inline-block text-sm font-medium" href={resolve("/docs/console")}>The console →</a>
      </div>
      <Shot src="shots/runs.png" alt="The current runs list filtered to two completed implementation runs, with their latest activity, cost, elapsed time, sessions, and turns." />
    </div>
  </section>

  <!-- Roles. -->
  <section class="border-border border-t">
    <div class="shell grid gap-10 py-20 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] lg:items-start">
      <div>
        <p class="eyebrow mb-4">Curated roles</p>
        <h2 class="font-serif text-4xl leading-[1.05] font-medium tracking-[-0.03em]">Name the work. Bind the model later.</h2>
        <p class="text-muted-foreground mt-5 text-lg leading-8">A workflow asks for <code class="text-foreground">sprint-planning</code> or <code class="text-foreground">code-review</code>, never a model. One file binds each role to a harness, model, and effort, and any run can override a role with a flag.</p>
        <a class="text-primary mt-6 inline-block text-sm font-medium" href={resolve("/docs/roles")}>The role catalog →</a>
      </div>
      <div class="frame">
        <div class="frame-bar"><i></i><i></i><i></i><span>internal/builtin/defaults.json</span></div>
        <dl class="divide-border divide-y font-mono text-[0.8rem]">
          {#each bindings as [role, model]}
            <div class="flex items-center justify-between gap-4 px-5 py-2">
              <dt class="text-foreground">{role}</dt>
              <dd class="text-primary">{model}</dd>
            </div>
          {/each}
        </dl>
      </div>
    </div>
  </section>

  <!-- Built-in workflows and harnesses. -->
  <section class="border-border border-t">
    <div class="shell grid gap-12 py-20 lg:grid-cols-2">
      <div>
        <p class="eyebrow mb-4">Built-in workflows</p>
        <h2 class="font-serif text-3xl leading-[1.08] font-medium tracking-[-0.03em]">Useful before you write one.</h2>
        <p class="text-muted-foreground mt-4 leading-7">Each command is generated from its workflow's source: a flag per parameter, a model flag per role.</p>
        <div class="frame code mt-6">
          <div class="frame-bar"><i></i><i></i><i></i><span>terminal</span></div>
          <RunHelp />
        </div>
        <a class="text-primary mt-6 inline-block text-sm font-medium" href={resolve("/docs/built-in")}>Built-in workflows →</a>
      </div>
      <div>
        <p class="eyebrow mb-4">Any harness</p>
        <h2 class="font-serif text-3xl leading-[1.08] font-medium tracking-[-0.03em]">Codex, Claude Code, Antigravity, or yours.</h2>
        <p class="text-muted-foreground mt-4 leading-7">Gimble drives coding-agent harnesses; it does not replace them. An adapter is five methods, and one run can mix harnesses by role.</p>
        <div class="frame code mt-6">
          <div class="frame-bar"><i></i><i></i><i></i><span>harness.go</span></div>
          <AdapterCode />
        </div>
        <a class="text-primary mt-6 inline-block text-sm font-medium" href={resolve("/docs/harnesses")}>Harness adapters →</a>
      </div>
    </div>
  </section>

  <section class="border-border border-t">
    <div class="shell py-20 text-center">
      <h2 class="mx-auto max-w-2xl font-serif text-4xl leading-[1.05] font-medium tracking-[-0.03em]">Start with the review workflow on your own repository.</h2>
      <div class="mx-auto mt-8 flex max-w-xl flex-col gap-3 sm:flex-row sm:items-center">
        <div class="min-w-0 flex-1 text-left"><InstallPrompt /></div>
        <a class="bg-primary text-primary-foreground inline-flex h-12 shrink-0 items-center justify-center rounded-lg px-5 text-sm font-semibold transition-opacity hover:opacity-90" href={resolve("/docs/quickstart")}>Quickstart →</a>
      </div>
    </div>
  </section>
</main>
