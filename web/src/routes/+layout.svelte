<script lang="ts">
  import "../app.css";
  import { untrack } from 'svelte';
  import type { WindowState } from '#lib/skgo/host/types';
  import { rememberWindow } from '#lib/window.remote';
  import { MediaQuery } from "svelte/reactivity";
  import Account from "#lib/auth/Account.svelte";
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { navigating, page } from "$app/state";
  import { APP_NAME } from "#lib/app.js";
  import ProjectWorkspace from "#lib/workspace/ProjectWorkspace.svelte";
  import RecorderBridge from "#lib/workspace/RecorderBridge.svelte";
  import favicon from "#lib/assets/favicon.svg";
  import RunsLoading from "#lib/run/RunsLoading.svelte";

  let { children, data } = $props();
  let windowState = $state<WindowState>(untrack(() => data.window));
  let saving = Promise.resolve();
  let saveError = $state('');
  function moved(place: WindowState) {
    const root = `/projects/${place.project}/`;
    windowState = { ...place, seen: [...(windowState.seen ?? []).filter(item => !item.path.startsWith(root)), ...place.seen] };
    saving = saving.then(async () => {
      try { await rememberWindow(place); saveError = ''; }
      catch (error) { if (!navigateOnUnauthorized(error)) saveError = 'Workspace preferences could not be saved.'; }
    });
  }
  const darkAppearance = new MediaQuery("prefers-color-scheme: dark", false);
  $effect(() => {
    document.documentElement.classList.toggle("dark", darkAppearance.current);
  });
  const project = $derived(page.params.project);
  const readerWindow = $derived(page.route.id === '/projects/[project]/files/[fileID]/window');
  const root = $derived(`/projects/${project}`);
  const loadingRuns = $derived(
    navigating.to?.url.pathname === root &&
      navigating.from?.url.pathname !== root,
  );
  const row =
    "flex h-8 items-center gap-2 rounded-lg px-2.5 hover:bg-sidebar-accent aria-[current=page]:bg-sidebar-accent";
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
{#if data.authEnabled && !data.user}
  {@render children()}
{:else if page.url.pathname.startsWith('/settings/')}
  <svelte:boundary onerror={(error) => { navigateOnUnauthorized(error); }}>
    {@render children()}
    {#snippet failed(error, reset)}<p role="alert" class="p-6">Could not load settings. <button onclick={reset}>Retry</button></p>{/snippet}
  </svelte:boundary>
{:else}
<svelte:boundary onerror={(error) => { navigateOnUnauthorized(error); }}>
<div class="flex h-screen">
  {#if !project}
    <aside
      aria-label={`${APP_NAME} sidebar`}
      class="flex w-[260px] flex-none flex-col overflow-auto border-r border-sidebar-border bg-sidebar px-2 pb-3 text-[13px] text-sidebar-foreground max-[900px]:w-[220px]"
    >
      <!-- The title bar's height, clear of the macOS traffic lights, as in the project sidebar. -->
      <div class="h-13 flex-none"></div>
      <div class="flex h-9 flex-none items-center gap-2.5 px-2 text-sm font-semibold">
        <span
          class="grid size-[22px] place-items-center rounded-md bg-sidebar-primary text-xs text-sidebar-primary-foreground"
          aria-hidden="true">{APP_NAME.slice(0, 1)}</span
        >{APP_NAME}
      </div>
      <nav aria-label={APP_NAME} class="mt-5 grid gap-px">
        <a
          class={row}
          href="/"
          aria-current={page.url.pathname === "/" ? "page" : undefined}>Projects</a
        >
        <a
          class={row}
          href="/workflows"
          aria-current={page.url.pathname.startsWith("/workflows")
            ? "page"
            : undefined}>Workflows</a
        >
      </nav>
      <h2 class="mx-2.5 mt-5 mb-1.5 text-xs font-medium text-muted-foreground">
        Open project
      </h2>
      {#each data.workspaceProjects as item (item.id)}<a
          class={row}
          href={`/projects/${item.id}/recordings`}
          title={item.path}
          ><span class="truncate">{item.path.split("/").at(-1)}</span></a
        >{:else}<p class="px-2.5 text-xs text-muted-foreground">
          No projects yet
        </p>{/each}
    </aside>
  {/if}
  <div class="flex min-w-0 flex-1 flex-col">
    {#if data.user}<Account name={data.user.name} />{/if}
    <main class="min-h-0 flex-1 overflow-auto">
      {#if saveError}<p role="alert">{saveError} <button onclick={() => moved(windowState)}>Retry</button></p>{/if}
      {#if project && !readerWindow}
        {#key project}<ProjectWorkspace
            {project}
            name={data.workspaceProjects
              .find((item: { id: string }) => item.id === project)
              ?.path.split("/")
              .at(-1) ?? "Project"}
            projects={data.workspaceProjects}
            restored={windowState.project === project ? windowState.tabs : []}
            sidebarPreference={windowState.sidebar}
            seenReplies={windowState.seen ?? []}
            onplace={moved}
          >
            {#if loadingRuns}
              <div class="runs-page"><RunsLoading /></div>
            {:else}
              {@render children()}
            {/if}
          </ProjectWorkspace>{/key}
      {:else}
        {@render children()}
      {/if}
    </main>
    <RecorderBridge dock={!project} />
  </div>
</div>

  {#snippet failed(error, reset)}
    <p role="alert" class="p-6">Could not load this view. <button onclick={reset}>Retry</button></p>
  {/snippet}
</svelte:boundary>
{/if}

<style>
  :global(body) {
    margin: 0;
    font: 16px/1.5 var(--font-sans);
    color: var(--foreground);
    background: var(--background);
  }

  .runs-page {
    display: flex;
    min-width: 0;
    justify-content: center;
    padding: 0 32px;
  }
</style>
