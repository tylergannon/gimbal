<script lang="ts">
  import type { Snippet } from 'svelte';
  import { MediaQuery } from 'svelte/reactivity';
  import { PanelLeft } from '@lucide/svelte';
  import TabStrip from './TabStrip.svelte';
  import TabGlyph from './TabGlyph.svelte';
  import TabStatus from './TabStatus.svelte';
  import type { WorkspaceTab, RecordingDisplay } from './types.js';

  let { project, items, tabs, active, sidebar = $bindable(false), recording, content,
    sidebarExtra, sidebarFooter, fallback, native = false, onproject, onrecording, oncontrol, onselect, onclose, onopen, onnew }: {
    project: string;
    /** Project inventory, including views that are currently closed. */
    items: WorkspaceTab[];
    tabs: WorkspaceTab[];
    active?: string;
    sidebar?: boolean;
    recording?: RecordingDisplay;
    content: Snippet<[WorkspaceTab]>;
    sidebarExtra?: Snippet;
    sidebarFooter?: Snippet;
    fallback?: Snippet;
    native?: boolean;
    onproject?: (event: MouseEvent) => void;
    onrecording?: () => void;
    oncontrol?: (action: 'pause' | 'resume' | 'stop') => void;
    onselect: (id: string) => void;
    onclose: (id: string) => void;
    onopen: (id: string) => void;
    onnew?: () => void;
  } = $props();

  // Below 1100 px the sidebar overlays the views, so it gets out of the way like a popover.
  const overlay = new MediaQuery('width < 1100px', false);
  let aside = $state<HTMLElement>();
  let toggle = $state<HTMLElement>();
  function dismiss(event: MouseEvent) {
    if (!sidebar || !overlay.current || !(event.target instanceof Element)) return;
    const inside = aside?.contains(event.target);
    if (toggle?.contains(event.target) || (inside && !event.target.closest('a, .item'))) return;
    sidebar = false;
  }
</script>

<svelte:window onclick={dismiss} onkeydown={event => { if (event.key === 'Escape' && sidebar && overlay.current) sidebar = false; }} />

{#snippet recordingStatus()}
  {#if recording}
    <div class="recording-status" role="status" aria-label={`${recording.state}: ${recording.label}`}>
      <button class="recording-open" type="button" onclick={onrecording} title={recording.detail} aria-label={`Open recording in ${recording.owner ?? project}, ${recording.label}`}>
        <span class="record-dot {recording.state}" aria-hidden="true"></span>
        {#if sidebar}<span>{recording.state === 'recording' ? 'Recording' : recording.state === 'paused' ? 'Paused' : recording.state === 'saving' ? 'Saving…' : 'Error'}</span>{/if}
        <span class="timer">{recording.label}</span>
      </button>
      {#if recording.canPause}<button class="recording-action" onclick={() => oncontrol?.('pause')}>Pause</button>{:else if recording.canResume}<button class="recording-action" onclick={() => oncontrol?.('resume')}>Resume</button>{/if}
      {#if sidebar && recording.canStop}<button class="recording-action" onclick={() => oncontrol?.('stop')}>Stop</button>{/if}
    </div>
  {/if}
{/snippet}

<div class="shell" class:with-sidebar={sidebar} class:native aria-label={`${project} workspace`}>
  {#if sidebar}
    <aside aria-label="Project sidebar" bind:this={aside}>
      <!-- Native titlebar integration supplies the traffic lights later. -->
      <div class="native-space"></div>
      <button class="project" type="button" onclick={onproject} aria-label={`Switch project, ${project}`}><span class="tile" aria-hidden="true">{project.slice(0, 1).toUpperCase()}</span><span class="project-name" title={project}>{project}</span><span aria-hidden="true">⌃⌄</span></button>
      {@render recordingStatus()}
      {#each ['chat', 'file'] as kind}
        <h2>{kind === 'chat' ? 'Chats' : 'Files'}</h2>
        {#each items.filter(item => item.kind === kind) as tab (tab.id)}
          <button class="item" class:selected={active === tab.id} type="button" title={tab.title} onclick={() => onopen(tab.id)}>
            <TabGlyph {tab} /><span class="item-title">{tab.title}</span><TabStatus status={tab.status} />
            {#if tab.detail}<span class="detail">{tab.detail}</span>{/if}
          </button>
        {/each}
        {#if kind === 'chat' && onnew}<button class="item new-chat" onclick={onnew}>New chat</button>{/if}
      {/each}
      {#if sidebarExtra}{@render sidebarExtra()}{/if}
      {#if sidebarFooter}<div class="sidebar-footer">{@render sidebarFooter()}</div>{/if}
    </aside>
  {/if}
  <section class="workspace" aria-label="Open views">
    <!-- This is a tab strip. The menubar is the macOS system menubar. -->
    <header>
      {#if !sidebar}<button class="compact-project" type="button" onclick={onproject} aria-label={`Switch project, ${project}`} title={project}>{project} <span aria-hidden="true">⌃⌄</span></button>{/if}
      <TabStrip {tabs} {active} {onselect} {onclose} {onnew} />
      {#if !sidebar}{@render recordingStatus()}{/if}
      <button bind:this={toggle} class="sidebar-toggle" class:on={sidebar} type="button" aria-label={sidebar ? 'Hide sidebar' : 'Show sidebar'} aria-expanded={sidebar} onclick={() => sidebar = !sidebar}><PanelLeft size={16} aria-hidden="true" /></button>
    </header>
    <div class="panels">
      {#each tabs as tab (tab.id)}
        <!-- Retain the supplied component and its DOM while switching views. -->
        <div class="panel" role="tabpanel" id={`workspace-panel-${tab.id}`} aria-label={tab.title} hidden={active !== tab.id} tabindex="0">
          {@render content(tab)}
        </div>
      {/each}
      {#if !active}{#if fallback}<div class="panel">{@render fallback()}</div>{:else}<div class="empty">Open a chat or file from the sidebar.</div>{/if}{/if}
    </div>
  </section>
</div>

<style>
  .shell { position: relative; display: flex; height: 100%; min-height: 0; overflow: hidden; background: var(--background); color: var(--foreground); font: 14px var(--font-sans); }
  .native:not(.with-sidebar) header { padding-left: 86px; }
  .native header { padding-right: 78px; }
  .workspace { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; }
  header { display: flex; align-items: center; flex: none; gap: 8px; height: 52px; padding: 0 14px; border-bottom: 1px solid var(--border); }
  .compact-project { flex: none; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 0 8px; font-size: 13px; font-weight: 600; }
  button { font: inherit; border: 0; background: transparent; color: inherit; cursor: pointer; }
  button:focus-visible { outline: 2px solid var(--ring); outline-offset: -2px; }
  .sidebar-toggle { display: flex; align-items: center; justify-content: center; flex: none; width: 28px; height: 28px; border-radius: 8px; color: var(--muted-foreground); }
  .sidebar-toggle:hover, .sidebar-toggle.on { background: var(--muted); color: var(--foreground); }
  .panels { flex: 1; min-height: 0; position: relative; }
  .panel { height: 100%; min-height: 0; }
  .panel[hidden] { display: none; }
  .panel:focus-visible { outline: 2px solid var(--ring); outline-offset: -2px; }
  .empty { display: grid; place-items: center; height: 100%; color: var(--muted-foreground); }
  aside { width: 260px; flex: none; display: flex; flex-direction: column; min-height: 0; overflow: auto; padding: 0 8px 12px; background: var(--sidebar); color: var(--sidebar-foreground); border-right: 1px solid var(--sidebar-border); }
  .native-space { height: 52px; flex: none; }
  .project { display: flex; align-items: center; gap: 8px; min-height: 38px; text-align: left; flex: none; padding: 0 10px; font-size: 13px; font-weight: 500; }
  .project-name { flex: 1; }
  .new-chat { color: var(--muted-foreground); padding-left: 32px; }
  .sidebar-footer { margin-top: auto; padding-top: 20px; }
  .project span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tile { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; background: var(--sidebar-primary); color: var(--sidebar-primary-foreground); font-size: 12px; }
  .recording-status { display: flex; align-items: center; gap: 8px; padding: 0 8px; height: 32px; flex: none; font-size: 13px; color: var(--muted-foreground); }
  .recording-open { display: flex; align-items: center; gap: 8px; padding: 0; min-width: 0; }
  .recording-action { font-size: 12px; padding: 4px 6px; border-radius: 5px; }
  .recording-action:hover { background: var(--muted); }
  .timer { font: 12px var(--font-mono); font-variant-numeric: tabular-nums; }
  .record-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--destructive); flex: none; }
  .record-dot.paused, .record-dot.error { background: transparent; border: 2px solid var(--destructive); }
  .record-dot.saving { animation: saving 1.4s infinite; }
  h2 { margin: 20px 10px 6px; font-size: 12px; font-weight: 500; color: var(--muted-foreground); }
  .item { display: flex; align-items: center; gap: 8px; padding: 0 10px; height: 32px; flex: none; border-radius: 8px; text-align: left; font-size: 13px; }
  .item-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .item:hover, .item.selected { background: var(--sidebar-accent); }
  .detail { font-size: 11px; color: var(--muted-foreground); }
  @keyframes saving { 50% { opacity: .4; } }
  @media (prefers-reduced-motion: reduce) { .record-dot.saving { animation: none; } }
  @media (width < 1100px) {
    aside { position: absolute; inset: 0 auto 0 0; z-index: 20; box-shadow: var(--shadow-lg); }
    .native header { padding-left: 86px; }
    .with-sidebar header { padding-left: 274px; }
  }
</style>
