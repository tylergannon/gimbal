<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { onMount, untrack, type Snippet } from 'svelte';
  import { MediaQuery } from 'svelte/reactivity';
  import * as Dialog from '#lib/components/ui/dialog';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { afterNavigate, goto } from '$app/navigation';
  import { page } from '$app/state';
  import WorkspaceShell from '#lib/shell/WorkspaceShell.svelte';
  import type { WorkspaceTab, RecordingDisplay } from '#lib/shell/types';
  import { projectConversations } from '../../routes/conversation.remote';
  import { projectFiles, uploadFile } from '../workspace.remote';
  import { rememberWindow } from '../window.remote';
  import type { WindowTab } from '#lib/skgo/host/types';
  import { recordingHistory } from '../transcript.remote';
  import ConversationView from '../conversation/ConversationView.svelte';
  import NewSession from '../conversation/NewSession.svelte';
  import RecordingTranscript from './RecordingTranscript.svelte';
  import FileViewer from './FileViewer.svelte';
  import PromiseLedger from './PromiseLedger.svelte';
  import { nativeMessage, type NativeStatus } from './native';
  import { conversationsForWorkspace } from './conversations';
  import OpenFolder from './OpenFolder.svelte';

  let { project, name, projects, restored = [], children }: {
    project: string; name: string; projects: { id: string; path: string }[]; restored?: WindowTab[]; children: Snippet;
  } = $props();
  const root = $derived(`/projects/${project}`);
  const chats = $derived(projectConversations(project));
  const documents = $derived(projectFiles(project));
  let after = $state('');
  const recordings = $derived(recordingHistory({ project, after }));
  const newest = $derived(recordingHistory({ project, after: '' }));
  const routeConversations = $derived((page.data as { items?: Conversation[]; selected?: Conversation }).items?.filter(item => typeof item.provider === 'string') ?? []);
  const routeSelected = $derived((page.data as { selected?: Conversation }).selected);
  const conversations = $derived(conversationsForWorkspace(chats.current ?? [], routeConversations, routeSelected));
  const files = $derived(documents.current ?? []);
  const history = $derived(recordings.current ?? { items: [], next: '' });
  const initialConversation = (page.data as { selected?: Conversation }).selected;
  const initialPath = page.url.pathname;
  const initialTab: WorkspaceTab | undefined = initialConversation?.id
    ? { id: `${initialPath.split("/").slice(0, 3).join("/")}/conversations/${initialConversation.id}`, title: initialConversation.title, kind: 'chat' }
    : page.params.fileID ? { id: initialPath, title: 'Project file', kind: 'file' }
    : page.params.recordingID ? { id: initialPath, title: 'Recorded conversation', kind: 'chat' }
    : initialPath.endsWith('/promises') ? { id: initialPath, title: 'Promises', kind: 'file' }
    : initialPath.endsWith('/conversations/new') ? { id: initialPath, title: 'New session', kind: 'chat' } : undefined;
  const upload = $derived(uploadFile.for(project));
  // Keyed by project, so the desktop window's last tabs seed this mount only.
  const reopened = untrack(() => restored).map(tab => ({ ...tab, kind: tab.kind === 'file' ? 'file' as const : 'chat' as const }));
  let open = $state<WorkspaceTab[]>([...reopened, ...(initialTab && !reopened.some(tab => tab.id === initialTab.id) ? [initialTab] : [])]);
  // The desktop window opens at 1000 px, so its server render starts without the sidebar.
  const wide = new MediaQuery('min-width: 1100px', !(page.data as { desktop?: boolean }).desktop);
  let sidebar = $derived(wide.current);
  let native = $state(false);
  let status = $state<NativeStatus>();
  const active = $derived(open.find(tab => tab.id === page.url.pathname)?.id ?? (page.url.pathname === `${root}/conversations` ? open[0]?.id : undefined));
  const items = $derived<WorkspaceTab[]>([
    ...conversations.map(item => ({ id: `${root}/conversations/${item.id}`, title: item.title,
      kind: 'chat' as const, detail: item.provider === 'codex' ? 'Codex' : item.provider === 'agy' ? 'agy / Gemini' : 'Claude',
      status: ['working', 'stopping', 'unresolved'].includes(item.status) || item.runs.some(run => run.status === 'running') ? 'working' as const : item.error ? 'attention' as const : undefined })),
    ...files.map(file => ({ id: `${root}/files/${file.id}`, title: file.name, kind: 'file' as const })),
  ]);
  const tabs = $derived(open.map(tab => items.find(item => item.id === tab.id) ?? tab));
  const recording = $derived.by((): RecordingDisplay | undefined => {
    if (!status || status.project !== project || !status.sessionID || ['idle', 'stopped'].includes(status.state)) return;
    return { state: status.error ? 'error' : status.pending ? 'saving' : status.state === 'paused' ? 'paused' : 'recording', label: status.error ? 'Recording error' : status.pending ? 'Saving…' : status.state };
  });

  function visit(path: string) {
    const item = items.find(item => item.id === path);
    const recordingID = page.params.recordingID;
    const tab = item ?? (path === `${root}/conversations/new`
      ? { id: path, title: 'New session', kind: 'chat' as const }
      : path === `${root}/promises` ? { id: path, title: 'Promises', kind: 'file' as const }
      : page.params.fileID ? { id: path, title: files.find(file => file.id === page.params.fileID)?.name ?? 'Project file', kind: 'file' as const }
      : recordingID ? { id: path, title: 'Recorded conversation', kind: 'chat' as const } : undefined);
    // A conversation started from the New conversation tab takes over that tab.
    const form = path === started ? open.findIndex(item => item.id === `${root}/conversations/new`) : -1;
    started = undefined;
    if (!tab || open.some(item => item.id === tab.id)) return;
    open = form >= 0 ? open.with(form, tab) : [...open, tab];
  }
  let actionError = $state("");
  let fileChooser = $state(false);
  let projectChooser = $state(false);
  const todayRecording = $derived(newest.current?.items.find(item => new Date(item.created).toDateString() === new Date().toDateString()));
  let started: string | undefined;
  function updated(conversation: Conversation) { chats.set([conversation, ...conversations.filter(item => item.id !== conversation.id)]); }
  function created(conversation: Conversation) {
    chats.set([conversation, ...conversations.filter(item => item.id !== conversation.id)]);
    started = `${root}/conversations/${conversation.id}`;
  }
  afterNavigate(() => {
    if (page.url.pathname === `${root}/conversations` && initialConversation?.id) select(`${root}/conversations/${initialConversation.id}`);
    else visit(page.url.pathname);
  });
  function select(path: string) { void goto(path, { reset: false }); }
  function close(path: string) {
    const index = open.findIndex(tab => tab.id === path);
    open = open.filter(tab => tab.id !== path);
    if (page.url.pathname === path) select(open[Math.min(index, open.length - 1)]?.id ?? `${root}/recordings`);
  }
  function command(event: Event) {
    const { command: value, project: targetProject } = (event as CustomEvent<{ command: string; project?: string }>).detail;
    if (value === 'openRecording') {
      const current = window.voiceNotesLatestStatus ?? status;
      if (current?.project && current.sessionID) select(`/projects/${current.project}/recordings/${current.sessionID}`);
    }
    else if (value === 'newSession') select(`${root}/conversations/new`);
    else if (value === 'openFileChooser') fileChooser = true;
    else if (value === 'switchProject') {
      if (targetProject && projects.some(item => item.id === targetProject)) select(`/projects/${targetProject}/recordings`);
      else projectChooser = true;
    }
    else if (value === 'showRuns') select(root);
    else if (value === 'openTodayTranscript' && todayRecording) select(`${root}/recordings/${todayRecording.id}`);
    else if (value === 'toggleSidebar') sidebar = !sidebar;
    else if (value === 'closeTab' && active) close(active);
    else if (value === 'nextTab' || value === 'previousTab') {
      const index = tabs.findIndex(tab => tab.id === active);
      const next = tabs[(index + (value === 'nextTab' ? 1 : -1) + tabs.length) % tabs.length];
      if (next) select(next.id);
    }
  }
  $effect(() => {
    if (!native) return;
    nativeMessage('workspaceState', { ready: true, sidebarVisible: sidebar, tabCount: tabs.length, activeTabTitle: tabs.find(tab => tab.id === active)?.title, canStopResponse: conversations.some(item => `${root}/conversations/${item.id}` === active && !!item.active_request && item.status !== 'stopping'), projectName: name, projects: projects.map(item => ({ id: item.id, name: item.path.split('/').at(-1) ?? item.path })), canOpenTodayTranscript: !!todayRecording });
  });
  const place = $derived(JSON.stringify({ project, path: page.url.pathname, tabs: tabs.map(({ id, title, kind }) => ({ id, title, kind })) }));
  $effect(() => {
    const value = JSON.parse(place);
    if (native) untrack(() => void rememberWindow(value).catch(() => {}));
  });
  $effect(() => {
    if (!conversations.some(item => ['working', 'stopping', 'unresolved'].includes(item.status) || item.runs.some(run => run.status === 'running'))) return;
    const timer = setInterval(() => { void projectConversations(project).refresh().catch(navigateOnUnauthorized); }, 1000);
    return () => clearInterval(timer);
  });
  $effect(() => {
    for (const query of [chats, documents, recordings, newest]) {
      if (query.error) void query.catch(navigateOnUnauthorized);
    }
  });
  onMount(() => {
    native = !!window.webkit?.messageHandlers?.workspaceState;
    window.addEventListener("gimbal-workspace-command", command);
    status = window.voiceNotesLatestStatus;
    const timer = setInterval(() => {
      status = window.voiceNotesLatestStatus;
      void recordingHistory({ project, after: '' }).refresh().catch(navigateOnUnauthorized);
      if (after) void recordingHistory({ project, after }).refresh().catch(navigateOnUnauthorized);
    }, 5000);
    return () => { window.removeEventListener("gimbal-workspace-command", command); clearInterval(timer); if (native) nativeMessage('workspaceState', { ready: false, sidebarVisible: false, tabCount: 0 }); };
  });
</script>

<WorkspaceShell project={name} {items} {tabs} {active} bind:sidebar {recording} {native}
  onselect={select} onclose={close} onopen={select} onnew={() => select(`${root}/conversations/new`)}>
  {#snippet content(tab)}
    <svelte:boundary onerror={(error) => { navigateOnUnauthorized(error); }}>
      {#if tab.id === `${root}/conversations/new`}<NewSession {project} {name} oncreated={created} />
      {:else if tab.id.startsWith(`${root}/conversations/`)}
        {@const current = conversations.find(item => `${root}/conversations/${item.id}` === tab.id)}
        {#if current}<ConversationView {project} {current} active={active === tab.id} onstate={updated} />{:else}<p class="notice">Conversation unavailable.</p>{/if}
      {:else if tab.id.startsWith(`${root}/files/`)}<FileViewer {project} id={tab.id.split('/').at(-1)!} />
      {:else if tab.id === `${root}/promises`}<PromiseLedger {project} recordings={history.items} />
      {:else}<RecordingTranscript {project} id={tab.id.split('/').at(-1)!} />{/if}
      {#snippet failed(error, reset)}<p class="notice" role="alert">Could not load this view. <button onclick={reset}>Retry</button></p>{/snippet}
    </svelte:boundary>
  {/snippet}
  {#snippet fallback()}
    {#if page.url.pathname === `${root}/conversations`}
      <NewSession {project} {name} oncreated={created} />
    {:else if page.url.pathname === `${root}/recordings`}
      <section class="welcome"><p class="eyebrow">Project</p><h1>{name}</h1><p>Open a conversation or file from the sidebar, or start a new conversation.</p><button onclick={() => select(`${root}/conversations/new`)}>New conversation</button><p>Record audio from the macOS Recording menu.</p></section>
    {:else}<div class="route-content">{@render children()}</div>{/if}
  {/snippet}
  {#snippet sidebarExtra()}
    {#if actionError}<p role="alert">{actionError}</p>{/if}
    {#if native && status?.sessionID && status.project === project}<div class="recording-detail"><span>{status.pending ? 'Saving to server…' : status.state}</span>{#if status.error}<p role="alert">{status.error}</p><button onclick={() => nativeMessage('reconnect', {})}>Reconnect</button>{/if}</div>{/if}
    <h2 class="group">Recordings</h2>
    {#each history.items as item (item.id)}<a class="row" class:selected={page.params.recordingID === item.id} href={`${root}/recordings/${item.id}`}><span>◉</span><span>{new Date(item.created).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} · {new Date(item.created).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}</span><small>{item.status}</small></a>{:else}<p class="quiet">No recordings yet</p>{/each}
    <details class="upload"><summary>Add project file</summary><form {...upload.enhance(async (submitted) => { try { if (await submitted.submit().updates(projectFiles(project)) && submitted.result) { documents.set([submitted.result, ...files.filter(file => file.id !== submitted.result!.id)]); select(`${root}/files/${submitted.result.id}`); } } catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); } })}><input {...upload.fields.project.as('hidden', project)} /><input aria-label="Choose project file" {...upload.fields.file.as('file')} required /><button disabled={!!upload.pending}>{upload.pending ? 'Uploading…' : 'Upload'}</button>{#each upload.fields.allIssues() ?? [] as issue}<p role="alert">{issue.message}</p>{/each}</form></details>
    <div class="pager">{#if after}<button onclick={() => after = ''}>Newest</button>{/if}{#if history.next}<button onclick={() => after = history.next}>Older recordings</button>{/if}</div>
    <nav class="tools" aria-label="Project tools"><a href={root}>Runs</a><a href={`${root}/promises`}>Promises</a><a href="/workflows">Workflows</a><a href={`${root}/about`}>About project</a></nav>
    <details class="projects"><summary>Switch project</summary>{#each projects as item (item.id)}<a class="row" href={`/projects/${item.id}/recordings`}>{item.path.split('/').at(-1)}</a>{/each}{#if native}<OpenFolder class="hover:bg-sidebar-accent flex min-h-8 w-full items-center rounded-lg px-2.5 text-xs">Add project</OpenFolder>{:else}<a class="row" href="/">Add project</a>{/if}</details>
  {/snippet}
</WorkspaceShell>

<Dialog.Root bind:open={fileChooser}>
  <Dialog.Content>
    <Dialog.Header><Dialog.Title>Open project file</Dialog.Title><Dialog.Description>Choose a file already added to this project.</Dialog.Description></Dialog.Header>
    <div class="choices">
      {#each files as file (file.id)}<button onclick={() => { fileChooser = false; select(`${root}/files/${file.id}`); }}>{file.name}</button>{:else}<p>No project files yet. Add a project file from the sidebar.</p>{/each}
    </div>
  </Dialog.Content>
</Dialog.Root>
<Dialog.Root bind:open={projectChooser}>
  <Dialog.Content>
    <Dialog.Header><Dialog.Title>Switch project</Dialog.Title><Dialog.Description>Open a workspace from your project catalog.</Dialog.Description></Dialog.Header>
    <div class="choices">
      {#each projects as item (item.id)}<button onclick={() => { projectChooser = false; select(`/projects/${item.id}/recordings`); }}>{item.path.split('/').at(-1)}</button>{/each}
    </div>
  </Dialog.Content>
</Dialog.Root>

<style>
  .choices { display: grid; gap: 8px; }
  .choices button { padding: 10px; text-align: left; color: inherit; background: var(--muted); border: 0; border-radius: 8px; cursor: pointer; }
  .choices p { color: var(--muted-foreground); font-size: 13px; }
  .recording-detail { padding: 8px 10px; color: var(--muted-foreground); font-size: 12px; }
  .recording-detail p { color: var(--destructive); }
  .group { margin: 20px 10px 6px; color: var(--muted-foreground); font-size: 12px; font-weight: 500; }
  .row { display: flex; align-items: center; gap: 8px; min-height: 32px; padding: 0 10px; border-radius: 8px; color: inherit; font-size: 12px; text-decoration: none; }
  .row.selected, .row:hover { background: var(--sidebar-accent); }
  .row small { margin-left: auto; color: var(--muted-foreground); font-size: 10px; }
  .quiet { padding: 0 10px; color: var(--muted-foreground); font-size: 12px; }
  .pager { display: flex; gap: 12px; padding: 0 10px; }
  .pager button { padding: 4px 0; border: 0; background: transparent; color: var(--muted-foreground); font: inherit; font-size: 12px; cursor: pointer; }
  .pager button:hover { color: var(--foreground); }
  .tools { display: grid; gap: 8px; border-top: 1px solid var(--border); padding: 14px 10px; margin-top: 18px; }
  .tools a { color: var(--muted-foreground); text-decoration: none; font-size: 13px; }
  .tools a:hover { color: var(--foreground); }
  .projects, .upload { padding: 8px 10px; font-size: 12px; }
  summary { cursor: pointer; color: var(--muted-foreground); }
  .upload form { display: grid; gap: 8px; padding-top: 8px; }
  .upload input { width: 100%; font-size: 11px; }
  .notice { padding: 24px; color: var(--muted-foreground); }
  .welcome { max-width: 620px; margin: 72px auto; padding: 24px; }
  .welcome .eyebrow { margin: 0; font-size: 12px; }
  .welcome h1 { margin-top: 2px; font-size: 24px; font-weight: 600; }
  .welcome p { color: var(--muted-foreground); }
  .welcome button { padding: 10px 16px; border: 0; border-radius: 8px; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .route-content { height: 100%; overflow: auto; }
</style>
