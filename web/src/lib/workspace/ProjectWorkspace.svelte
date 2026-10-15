<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { APP_NAME } from "#lib/app";
  import { errorMessage } from "#lib/errors";
  import { onMount, untrack, type Snippet } from 'svelte';
  import { MediaQuery, SvelteMap } from 'svelte/reactivity';
  import * as Dialog from '#lib/components/ui/dialog';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import { afterNavigate, goto } from '$app/navigation';
  import { page } from '$app/state';
  import WorkspaceShell from '#lib/shell/WorkspaceShell.svelte';
  import type { WorkspaceTab, RecordingDisplay } from '#lib/shell/types';
  import { projectConversations } from '../../routes/conversation.remote';
  import { projectFiles, uploadFile } from '../workspace.remote';
  import type { WindowTab, WindowState } from '#lib/skgo/host/types';
  import { recordingHistory } from '../transcript.remote';
  import ConversationView from '../conversation/ConversationView.svelte';
  import NewSession from '../conversation/NewSession.svelte';
  import RecordingTranscript from './RecordingTranscript.svelte';
  import FileViewer from './FileViewer.svelte';
  import PromiseLedger from './PromiseLedger.svelte';
  import ProjectRuns from './ProjectRuns.svelte';
  import type { RunCardItem } from '#lib/run/RunsList.svelte';
  import RunPanel from './RunPanel.svelte';
  import type { RunRow, RunSnapshot, InterviewRow } from '#lib/observation';
  import { nativeMessage, projectMenu, recordingControl, recordingTime, type NativeStatus } from './native';
  import { conversationsForWorkspace, conversationNeedsUpdates, completedReply } from './conversations';
  import { harnessName } from '#lib/conversation/names';

  let { project, name, projects, restored = [], sidebarPreference = null, seenReplies = [], onplace, children }: {
    project: string; name: string; projects: { id: string; path: string }[]; restored?: WindowTab[]; sidebarPreference?: boolean | null; seenReplies?: { path: string; request: string }[]; onplace?: (place: WindowState) => void; children: Snippet;
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
    : initialPath.endsWith('/discuss') ? { id: `${initialPath.split('/').slice(0, 3).join('/')}/conversations/new`, title: 'New session', kind: 'chat' }
    : page.params.fileID ? { id: initialPath, title: 'Project file', kind: 'file' }
    : page.params.recordingID ? { id: initialPath, title: 'Recorded conversation', kind: 'chat' }
    : initialPath.endsWith('/promises') ? { id: initialPath, title: 'Promises', kind: 'file' }
    : initialPath === initialPath.split("/").slice(0, 3).join("/") ? { id: initialPath, title: 'Runs', kind: 'run' }
    : page.params.runID && (page.data as { snapshot?: RunSnapshot }).snapshot ? { id: initialPath, title: (page.data as { snapshot?: { run: { name: string } } }).snapshot?.run.name ?? 'Run', kind: 'run' }
    : initialPath.endsWith('/conversations/new') ? { id: initialPath, title: 'New session', kind: 'chat' } : undefined;
  const upload = $derived(uploadFile.for(project));
  // Keyed by project, so the desktop window's last tabs seed this mount only.
  const reopened = untrack(() => restored).map(tab => ({ ...tab, kind: tab.kind === 'run' ? 'run' as const : tab.kind === 'file' ? 'file' as const : 'chat' as const }));
  let open = $state<WorkspaceTab[]>([...reopened, ...(initialTab && !reopened.some(tab => tab.id === initialTab.id) ? [initialTab] : [])]);
  // The desktop window opens at 1000 px, so its server render starts without the sidebar.
  const wide = new MediaQuery('min-width: 1100px', !(page.data as { desktop?: boolean }).desktop);
  let preferredSidebar = $state(untrack(() => sidebarPreference));
  const sidebar = $derived(preferredSidebar ?? wide.current);
  let native = $state(false);
  let status = $state<NativeStatus>();
  const active = $derived(page.status >= 400 ? undefined : open.find(tab => tab.id === page.url.pathname)?.id ?? (page.url.pathname === `${root}/conversations` ? open[0]?.id : undefined));
  const seen = new SvelteMap<string, string>(untrack(() => seenReplies.map(item => [item.path, item.request] as const)));
  $effect(() => {
    const item = conversations.find(item => `${root}/conversations/${item.id}` === active);
    if (item) seen.set(`${root}/conversations/${item.id}`, completedReply(item));
  });
  const items = $derived<WorkspaceTab[]>([
    ...conversations.map(item => ({ id: `${root}/conversations/${item.id}`, title: item.title,
      kind: 'chat' as const, detail: harnessName(item.provider),
      status: ['working', 'stopping', 'unresolved'].includes(item.status) || item.runs.some(run => run.status === 'running') ? 'working' as const : item.error ? 'attention' as const : completedReply(item) && completedReply(item) !== seen.get(`${root}/conversations/${item.id}`) ? 'unread' as const : undefined })),
    ...files.map(file => ({ id: `${root}/files/${file.id}`, title: file.name, kind: 'file' as const })),
  ]);
  let runTabs = $state<WorkspaceTab[]>([]);
  const tabs = $derived(open.map(tab => items.find(item => item.id === tab.id) ?? runTabs.find(item => item.id === tab.id) ?? tab));
  function runState(run: RunRow, waiting: number) {
    const id = `${root}/runs/${encodeURIComponent(run.id)}`;
    runTabs = [...runTabs.filter(tab => tab.id !== id), { id, title: run.name, kind: 'run', status: waiting ? 'attention' : run.status === 'running' ? 'working' : undefined }];
  }
  const recording = $derived.by((): RecordingDisplay | undefined => {
    if (!status?.sessionID || ['idle', 'stopped'].includes(status.state)) return;
    const state = status.state === 'paused' ? 'paused' : status.state === 'recording' ? 'recording' : status.state === 'error' ? 'error' : 'saving';
    const owner = projects.find(item => item.id === status?.project)?.path.split('/').at(-1) ?? 'Recording project';
    return { state, label: state === 'saving' ? status.state === 'preparing' ? 'Preparing…' : 'Saving…' : recordingTime(status.elapsedMs), owner, detail: `${owner} · ${status.source}${status.error ? ` · ${status.error}` : ''}`, canPause: status.canPause, canResume: status.canResume, canStop: status.canStop };
  });

  function dateGroup(created: number) {
    const date = new Date(created);
    const today = new Date();
    const yesterday = new Date(); yesterday.setDate(today.getDate() - 1);
    return date.toDateString() === today.toDateString() ? 'Today' : date.toDateString() === yesterday.toDateString() ? 'Yesterday' : date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: date.getFullYear() === today.getFullYear() ? undefined : 'numeric' });
  }
  function changedStatus(event: Event) { status = (event as CustomEvent<NativeStatus>).detail; }
  function openRecording() {
    if (status?.project && status.sessionID) select(`/projects/${status.project}/recordings/${status.sessionID}`);
  }
  function control(action: 'pause' | 'resume' | 'stop') { if (status) recordingControl(status, action); }
  function chooseProject(event: MouseEvent) { if (native) projectMenu(event); else projectChooser = true; }
  let fileContext = $state<{ id: string; name: string }>();
  function discuss(file: { id: string; name: string }) {
    fileContext = file;
    select(`${root}/conversations/new`);
  }
  let startLike = $state<{ conversation: string; nonce: number }>();
  let setupNonce = 0;
  function moveFile(path: string) {
    if (native) { nativeMessage('openFile', { path: `${path}/window` }); close(path); }
    else window.open(`${path}/window`, '_blank', 'noopener');
  }
  function visit(path: string) {
    if (page.status >= 400) return;
    const item = items.find(item => item.id === path);
    const recordingID = page.params.recordingID;
    const tab = item ?? (path === root ? { id: path, title: 'Runs', kind: 'run' as const }
      : page.params.runID && path.startsWith(`${root}/runs/`) ? { id: path, title: (page.data as { snapshot?: { run: { name: string } } }).snapshot?.run.name ?? 'Run', kind: 'run' as const }
      : path === `${root}/conversations/new`
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
  function created(conversation: Conversation) {
    chats.set([conversation, ...conversations.filter(item => item.id !== conversation.id)]);
    started = `${root}/conversations/${conversation.id}`;
    fileContext = undefined;
    startLike = undefined;
  }
  afterNavigate(() => {
    if (page.url.pathname.endsWith('/discuss') && page.params.fileID) {
      discuss({ id: page.params.fileID, name: files.find(file => file.id === page.params.fileID)?.name ?? 'Project file' });
      return;
    }
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
    const { command: value, project: targetProject, conversation, path } = (event as CustomEvent<{ command: string; project?: string; conversation?: string; path?: string }>).detail;
    if (value === 'openRecording') {
      const current = window.voiceNotesLatestStatus ?? status;
      if (current?.project && current.sessionID) select(`/projects/${current.project}/recordings/${current.sessionID}`);
    }
    else if (value === 'newSession') select(`${root}/conversations/new`);
    else if (value === 'newSessionLike' && conversation && conversations.some(item => item.id === conversation)) { startLike = { conversation, nonce: ++setupNonce }; select(`${root}/conversations/new`); }
    else if (value === 'selectTab' && path && tabs.some(tab => tab.id === path)) select(path);
    else if (value === 'moveTab' && active && files.some(file => active === `${root}/files/${file.id}`)) moveFile(active);
    else if (value === 'openFileChooser') fileChooser = true;
    else if (value === 'switchProject') {
      if (targetProject && projects.some(item => item.id === targetProject)) select(`/projects/${targetProject}/recordings`);
      else if (native) nativeMessage('projectMenu', {}); else projectChooser = true;
    }
    else if (value === 'showRuns') select(root);
    else if (value === 'openTodayTranscript' && todayRecording) select(`${root}/recordings/${todayRecording.id}`);
    else if (value === 'toggleSidebar') preferredSidebar = !sidebar;
    else if (value === 'closeTab' && active) close(active);
    else if (value === 'nextTab' || value === 'previousTab') {
      const index = tabs.findIndex(tab => tab.id === active);
      const next = tabs[(index + (value === 'nextTab' ? 1 : -1) + tabs.length) % tabs.length];
      if (next) select(next.id);
    }
  }
  $effect(() => {
    if (!native) return;
    nativeMessage('workspaceState', { project, ready: true, canMove: files.some(file => active === `${root}/files/${file.id}`), canRename: conversations.some(item => active === `${root}/conversations/${item.id}`), tabs: tabs.map(({ id, title }) => ({ id, title })), recentSessions: conversations.slice(0, 6).map(({ id, title }) => ({ id, title })), sidebarVisible: sidebar, tabCount: tabs.length, activeTabTitle: tabs.find(tab => tab.id === active)?.title, canStopResponse: conversations.some(item => `${root}/conversations/${item.id}` === active && !!item.active_request && item.status !== 'stopping'), projectName: name, projects: projects.map(item => ({ id: item.id, name: item.path.split('/').at(-1) ?? item.path, path: item.path })), canOpenTodayTranscript: !!todayRecording });
  });
  const place = $derived(JSON.stringify({ project, path: page.url.pathname, tabs: tabs.map(({ id, title, kind }) => ({ id, title, kind })), sidebar: preferredSidebar, seen: [...seen].filter(([path]) => path.startsWith(`${root}/conversations/`)).map(([path, request]) => ({ path, request })) }));
  $effect(() => {
    const value = JSON.parse(place);
    if (native) untrack(() => onplace?.(value));
  });
  $effect(() => {
    if (!conversations.some(conversationNeedsUpdates)) return;
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
    window.addEventListener("gimbal-recording-status", changedStatus);
    status = window.voiceNotesLatestStatus;
    const timer = setInterval(() => {
      void recordingHistory({ project, after: '' }).refresh().catch(navigateOnUnauthorized);
      if (after) void recordingHistory({ project, after }).refresh().catch(navigateOnUnauthorized);
    }, 5000);
    return () => { window.removeEventListener("gimbal-workspace-command", command); window.removeEventListener("gimbal-recording-status", changedStatus); clearInterval(timer); if (native) nativeMessage('workspaceState', { ready: false, sidebarVisible: false, tabCount: 0 }); };
  });
</script>

<svelte:head><title>{tabs.find(tab => tab.id === active)?.title ?? name} · {APP_NAME}</title></svelte:head>

<WorkspaceShell project={name} {items} {tabs} {active} bind:sidebar={() => sidebar, (value) => preferredSidebar = value} {recording} {native}
  onproject={chooseProject} onrecording={openRecording} oncontrol={control}
  onselect={select} onclose={close} onopen={select} onnew={() => select(`${root}/conversations/new`)}>
  {#snippet content(tab)}
    <svelte:boundary onerror={(error) => { navigateOnUnauthorized(error); }}>
      {#if tab.id === `${root}/conversations/new`}<NewSession {project} {name} {startLike} {fileContext} oncontextremoved={() => fileContext = undefined} active={active === tab.id} oncreated={created} />
      {:else if tab.id.startsWith(`${root}/conversations/`)}
        {@const current = conversations.find(item => `${root}/conversations/${item.id}` === tab.id)}
        {#if current}<ConversationView {project} {current} active={active === tab.id} />{:else}<p class="notice">Conversation unavailable.</p>{/if}
      {:else if tab.id.startsWith(`${root}/files/`)}<FileViewer {project} id={tab.id.split('/').at(-1)!} active={active === tab.id} ondiscuss={discuss} onmove={() => moveFile(tab.id)} />
      {:else if tab.id === root}<ProjectRuns {project} initial={page.url.pathname === root ? page.data as { items: RunCardItem[]; attention: InterviewRow[]; now: number } : undefined} onopen={select} onstate={value => runTabs = value} />
      {:else if tab.kind === 'run'}<RunPanel {project} initial={page.url.pathname === tab.id ? page.data as { snapshot: RunSnapshot; graph: string } : undefined} id={decodeURIComponent(tab.id.split('/').at(-1)!)} active={active === tab.id} onstate={runState} />
      {:else if tab.id === `${root}/promises`}<PromiseLedger {project} recordings={history.items} />
      {:else}<RecordingTranscript {project} id={tab.id.split('/').at(-1)!} active={active === tab.id} />{/if}
      {#snippet failed(error, reset)}<p class="notice" role="alert">Could not load this view. <button onclick={reset}>Retry</button></p>{/snippet}
    </svelte:boundary>
  {/snippet}
  {#snippet fallback()}
    {#if page.url.pathname === `${root}/conversations`}
      <NewSession {project} {name} {startLike} {fileContext} oncontextremoved={() => fileContext = undefined} oncreated={created} />
    {:else if page.url.pathname === `${root}/recordings`}
      <section class="welcome"><p class="eyebrow">Project</p><h1>{name}</h1><p>Open a conversation or file from the sidebar, or start a new conversation.</p><button onclick={() => select(`${root}/conversations/new`)}>New conversation</button><p>Record audio from the macOS Recording menu.</p></section>
    {:else}<div class="route-content">{@render children()}</div>{/if}
  {/snippet}
  {#snippet sidebarExtra()}
    {#if actionError}<p role="alert">{actionError}</p>{/if}
    <h2 class="group">Recordings</h2>
    {#each history.items as item, index (item.id)}
      {#if index === 0 || dateGroup(history.items[index - 1].created) !== dateGroup(item.created)}<h3 class="date-group">{dateGroup(item.created)}</h3>{/if}
      <a class="row" class:selected={page.params.recordingID === item.id} href={`${root}/recordings/${item.id}`} title={`${item.status} · ${item.source}`}><span class:live={status?.project === project && status.sessionID === item.id && status.state === 'recording'}>{status?.project === project && status.sessionID === item.id && status.state === 'paused' ? '◌' : '●'}</span><span>{new Date(item.created).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}</span><small>{recordingTime(status?.project === project && status.sessionID === item.id && !['idle', 'stopped'].includes(status.state) ? status.elapsedMs : item.durationMs)}</small></a>
    {:else}<p class="quiet">No recordings yet</p>{/each}
    <details class="upload"><summary>Add project file</summary><form {...upload.enhance(async (submitted) => { try { if (await submitted.submit().updates(projectFiles(project)) && submitted.result) { documents.set([submitted.result, ...files.filter(file => file.id !== submitted.result!.id)]); select(`${root}/files/${submitted.result.id}`); } } catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); } })}><input {...upload.fields.project.as('hidden', project)} /><input aria-label="Choose project file" {...upload.fields.file.as('file')} required /><button disabled={!!upload.pending}>{upload.pending ? 'Uploading…' : 'Upload'}</button>{#each upload.fields.allIssues() ?? [] as issue}<p role="alert">{issue.message}</p>{/each}</form></details>
    <div class="pager">{#if after}<button onclick={() => after = ''}>Newest</button>{/if}{#if history.next}<button onclick={() => after = history.next}>Older recordings</button>{/if}</div>
  {/snippet}
  {#snippet sidebarFooter()}
    <nav class="tools" aria-label="Project tools"><a class="row" href={root}>Runs</a><a class="row" href={`${root}/promises`}>Promises</a></nav>
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
  .group { margin: 20px 10px 6px; color: var(--muted-foreground); font-size: 12px; font-weight: 500; }
  .row { display: flex; align-items: center; gap: 8px; min-height: 32px; padding: 0 10px; border-radius: 8px; color: inherit; font-size: 12px; text-decoration: none; }
  .row.selected, .row:hover { background: var(--sidebar-accent); }
  .date-group { margin: 10px 10px 4px; font-size: 11px; color: var(--muted-foreground); font-weight: 400; }
  .live { color: var(--destructive); }
  .row small { margin-left: auto; color: var(--muted-foreground); font-size: 10px; }
  .quiet { padding: 0 10px; color: var(--muted-foreground); font-size: 12px; }
  .pager { display: flex; gap: 12px; padding: 0 10px; }
  .pager button { padding: 4px 0; border: 0; background: transparent; color: var(--muted-foreground); font: inherit; font-size: 12px; cursor: pointer; }
  .pager button:hover { color: var(--foreground); }
  .tools { display: grid; gap: 4px; }
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
