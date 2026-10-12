<script lang="ts">
  // Synthetic content for the shell's Storybook and browser behavior tests.
  import { ChevronRight, FileText, Paperclip, ArrowUp, AudioLines, Workflow } from '@lucide/svelte';
  import WorkspaceShell from './WorkspaceShell.svelte';
  import type { WorkspaceTab, RecordingDisplay } from './types.js';
  let { sidebar = false, many = false, file = false, recordingState = 'recording' }: {
    sidebar?: boolean; many?: boolean; file?: boolean; recordingState?: RecordingDisplay['state'];
  } = $props();
  const inventory: WorkspaceTab[] = [
    { id: 'layout', title: 'Workspace layout', kind: 'chat', detail: 'Claude' },
    { id: 'navigation', title: 'Return navigation #455', kind: 'chat', detail: 'Codex', status: 'working' },
    { id: 'webapp', title: 'web-app.md', kind: 'file' },
    { id: 'menu', title: 'Menu bar in Swift', kind: 'chat', detail: 'Claude', status: 'unread' },
    { id: 'map', title: 'run-map.md', kind: 'file', status: 'attention' },
    { id: 'api', title: 'API.md', kind: 'file' },
    ...Array.from({ length: 7 }, (_, i): WorkspaceTab => ({ id: `long${i}`, title: `Workspace research and implementation notes ${i + 1}.md`, kind: 'file' }))
  ];
  let open = $derived(inventory.slice(0, many ? inventory.length : 3));
  let active = $derived(file ? 'webapp' : 'layout');
  let shown = $derived(sidebar);
  function select(id: string) { active = id; }
  function close(id: string) {
    const index = open.findIndex(tab => tab.id === id);
    open = open.filter(tab => tab.id !== id);
    if (active === id) active = open[Math.min(index, open.length - 1)]?.id ?? '';
  }
  function reopen(id: string) {
    if (!open.some(tab => tab.id === id)) open = [...open, inventory.find(tab => tab.id === id)!];
    active = id;
  }
</script>

<div class="demo">
  <WorkspaceShell project="gimbal" items={inventory.slice(0, 6)} tabs={open} {active} bind:sidebar={shown}
    recording={{ state: recordingState, label: recordingState === 'error' ? 'Microphone lost' : recordingState === 'saving' ? 'Saving…' : '1:12:08' }}
    onselect={select} onclose={close} onopen={reopen} onnew={() => reopen('menu')}>
    {#snippet content(tab)}
      {#if tab.kind === 'chat'}
        <div class="chat-view">
          <div class="reading" aria-label={`${tab.title} transcript`}>
            <div class="column">
              <div class="user">Rethink the workspace shell. One tab bar for chats and files, the recording in the title bar, no footer. Keep the default window size usable.</div>
              <article>
                <div class="agent"><span class="tile">C</span>Claude Code · local worktree</div>
                <p>Three things carry the layout. The title bar owns the project, the tabs and the recording. The content area has nothing above it but one hairline. The sidebar becomes a toggle, so the default window keeps its full width for reading.</p>
                <div class="tools">
                  <div><ChevronRight size={14} /><code>Read</code><span>docs/web-app.md</span></div>
                  <div><ChevronRight size={14} /><code>Read</code><span>src/lib/shell/WorkspaceShell.svelte</span></div>
                  <div><ChevronRight size={14} /><code>Edited</code><span>WorkspaceShell.svelte</span><code>+42 −18</code></div>
                </div>
                <p>The tab strip now sits beside the project name. Chats and files use the same tab, told apart by a 14px glyph. A chat that is working shows a dot, nothing else.</p>
              </article>
              <div class="user">Good. What moves into Swift?</div>
              <article><div class="agent"><span class="tile">C</span>Claude Code · local worktree</div><p>The menubar means the macOS system menubar. Native application menus and titlebar integration follow in a separate slice.</p></article>
              {#each Array.from({ length: 6 }) as _, i}<article><div class="agent"><span class="tile">C</span>Claude Code · local worktree</div><p>Workspace note {i + 1}: switching tabs preserves your reading position and draft. Closing this view leaves agent work and recording under their current owner.</p></article>{/each}
            </div>
          </div>
          <div class="composer"><div class="box">
            <textarea aria-label={`Message for ${tab.title}`} placeholder="Message Claude…" rows="2"></textarea>
            <div class="composer-row"><Paperclip size={16} aria-hidden="true" /><code>claude/gimbal-workspace-layout-9242cd</code><span class="send" aria-hidden="true"><ArrowUp size={16} /></span></div>
          </div></div>
        </div>
      {:else}
        <div class="file-view">
          <div class="file-label"><FileText size={14} aria-hidden="true" />{tab.title}<span>Project file</span></div>
          <div class="document" aria-label={`${tab.title} document`}>
            <h1>Gimbal's web app</h1><p class="intro">One workspace for the project. Every chat, file and recording belongs here.</p>
            <h2>The workspace</h2><p>Chats and files share one tab strip. The sidebar reveals what the project owns; hiding it gives the reading column the full width of the window.</p>
            <h2>Quiet by default</h2><p>Two hairlines establish the structure. A small glyph tells a chat from a file, and a dot shows when a chat is working. Detail is available when you need it.</p>
            {#each Array.from({ length: 12 }) as _, i}<h2>Reading note {i + 1}</h2><p>Leave this document where you are reading. Switching to a chat and back retains this position.</p>{/each}
          </div>
        </div>
      {/if}
    {/snippet}
    {#snippet sidebarExtra()}
      <h2 class="group">Recordings</h2>
      <div class="sidebar-row"><span class="red-dot"></span>Today<span class="meta">1:12:08</span></div>
      <div class="sidebar-row"><AudioLines size={14} />Yesterday<span class="meta">42:10</span></div>
      <div class="runs"><Workflow size={14} />Runs<span class="meta">1 live</span></div>
    {/snippet}
  </WorkspaceShell>
</div>

<style>
  .demo { height: 100vh; }
  .chat-view, .file-view { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .reading { flex: 1; min-height: 0; overflow: auto; padding: 32px 24px 0; }
  .column { display: flex; flex-direction: column; gap: 26px; max-width: 720px; margin: 0 auto; padding-bottom: 16px; }
  .user { align-self: flex-end; max-width: min(560px, 100%); background: var(--muted); border-radius: 14px; padding: 10px 14px; line-height: 1.5; }
  article { display: flex; flex-direction: column; gap: 10px; }
  p { margin: 0; line-height: 1.625; }
  .agent { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 500; color: var(--muted-foreground); }
  .tile { display: grid; place-items: center; width: 20px; height: 20px; background: var(--primary); color: var(--primary-foreground); border-radius: 6px; font-size: 11px; font-weight: 600; }
  .tools { display: flex; flex-direction: column; gap: 2px; color: var(--muted-foreground); font-size: 13px; }
  .tools div { display: flex; align-items: center; gap: 8px; min-height: 26px; padding: 0 6px; }
  code { font: 12px var(--font-mono); overflow-wrap: anywhere; }
  .composer { flex: none; padding: 16px 24px 20px; }
  .box { max-width: 720px; margin: 0 auto; padding: 10px 10px 8px; background: var(--card); border: 1px solid var(--input); border-radius: 14px; box-shadow: var(--shadow-xs); }
  textarea { width: 100%; background: transparent; color: var(--foreground); border: 0; resize: none; padding: 4px 6px; font: inherit; line-height: 1.5; }
  textarea:focus-visible { outline: 2px solid var(--ring); border-radius: 6px; }
  .composer-row { display: flex; align-items: center; gap: 8px; color: var(--muted-foreground); padding-top: 8px; }
  .send { display: grid; place-items: center; margin-left: auto; flex: none; width: 28px; height: 28px; border-radius: 50%; background: var(--primary); color: var(--primary-foreground); }
  .file-label { display: flex; align-items: center; gap: 8px; padding: 16px 24px; font-size: 12px; color: var(--muted-foreground); }
  .file-label span { margin-left: auto; }
  .document { flex: 1; overflow: auto; padding: 8px max(24px, calc((100% - 720px) / 2)) 32px; }
  .document h1 { font-size: 28px; font-weight: 600; letter-spacing: -.03em; margin: 12px 0 10px; }
  .intro { color: var(--muted-foreground); }
  .document h2 { margin: 26px 0 8px; font-size: 18px; font-weight: 600; }
  .group { margin: 20px 10px 6px; font-size: 12px; font-weight: 500; color: var(--muted-foreground); }
  .sidebar-row, .runs { display: flex; align-items: center; gap: 8px; padding: 0 10px; height: 32px; flex: none; font-size: 13px; }
  .runs { margin-top: auto; padding-top: 18px; height: 50px; }
  .meta { margin-left: auto; font: 11px var(--font-mono); color: var(--muted-foreground); }
  .red-dot { width: 8px; height: 8px; background: var(--destructive); border-radius: 50%; }
</style>
