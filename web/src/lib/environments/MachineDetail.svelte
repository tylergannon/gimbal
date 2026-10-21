<script lang="ts">
  import { onMount } from 'svelte';
  import * as Dialog from '#lib/components/ui/dialog';
  import { reconnectMachine, removeMachine, watchMachineConnection, closeMachineConnection } from '#lib/environments.remote';
  import { cleanupConversation } from '../../routes/conversation.remote';
  import { errorMessage } from '#lib/errors';
  import type { MachineState, Snapshot, EnvironmentInfo } from '#lib/skgo/environments/types';
  import type { MachineConfiguration } from './configuration';
  import ConnectionChecks from './ConnectionChecks.svelte';
  import { timeLabel } from './navigation';
  let { machine, configuration, snapshot, onchanged, onedit, onadd, oneditenvironment }: { machine: MachineState; configuration?: MachineConfiguration; snapshot: Snapshot; onchanged: () => void; onedit: () => void; onadd: () => void; oneditenvironment: (environment: EnvironmentInfo) => void } = $props();
  const environments = $derived(snapshot.environments.filter(row => row.environment.machine === machine.id));
  const sessions = $derived(snapshot.sessions.filter(item => item.machine === machine.id && (item.active || item.pending)));
  let view = $state('');
  onMount(() => { view = crypto.randomUUID(); });
  const connection = $derived(view ? watchMachineConnection({ machine: machine.id, view }) : undefined);
  $effect(() => { const id = view; if (id) return () => { void closeMachineConnection(id).catch(() => {}); }; });
  const localBuild = $derived(snapshot.machines.find(item => item.id === 'local')?.build ?? '');
  const buildMismatch = $derived(machine.id !== 'local' && !!machine.build && machine.build !== 'unknown' && !!localBuild && localBuild !== 'unknown' && machine.build !== localBuild);
  let pending = $state(false);
  let error = $state('');
  let confirm = $state(false);
  let terminal = $state(false);
  let selected = $state<string[]>([]);
  async function reconnect() { pending = true; error = ''; try { await reconnectMachine(machine.id); onchanged(); } catch (reason) { error = errorMessage(reason); } finally { pending = false; } }
  async function remove() { pending = true; error = ''; try { await removeMachine({ id: machine.id, revision: snapshot.revision }); confirm = false; onchanged(); } catch (reason) { error = errorMessage(reason); } finally { pending = false; } }
  async function endSelected() {
    pending = true; error = '';
    try { for (const item of sessions.filter(item => selected.includes(`${item.project}/${item.conversation}`))) await cleanupConversation({ project: item.project, conversation: item.conversation }); onchanged(); }
    catch (reason) { error = errorMessage(reason); } finally { pending = false; }
  }
  function openTerminal() {
    const host = configuration?.ssh?.host;
    const handler = (window as Window & { webkit?: { messageHandlers?: { openMachineTerminal?: { postMessage: (body: { host: string; port: number; local?: boolean }) => void } } } }).webkit?.messageHandlers?.openMachineTerminal;
    if (machine.id === 'local' && handler) handler.postMessage({ host: '', port: 0, local: true });
    else if (host && handler) handler.postMessage({ host, port: configuration?.ssh?.port ?? 0 });
    else terminal = true;
  }
  function openSession(event: MouseEvent, path: string) {
    const handler = (window as Window & { webkit?: { messageHandlers?: { openSettingsSession?: { postMessage: (body: { path: string }) => void } } } }).webkit?.messageHandlers?.openSettingsSession;
    if (handler) { event.preventDefault(); handler.postMessage({ path }); }
  }
</script>

<section class="detail" aria-label={`${machine.name} details`}>
  <h2>{machine.name}</h2>
  <p class="status" class:trouble={machine.state === 'error' || machine.state === 'unreachable'}>{machine.state === 'connected' ? machine.kind === 'route' ? 'Worker reachable' : `Connected · ${machine.latency_ms} ms` : machine.state === 'not-started' ? 'Starts when needed' : machine.state} {#if machine.since}since {timeLabel(machine.since)}{/if}</p>
  {#if machine.detail}<p>{machine.detail}</p>{/if}
  <dl><dt>Gimbal build</dt><dd>{machine.build || 'Not observed yet'}</dd><dt>Connection</dt><dd>{configuration?.ssh?.host ? `SSH · ${configuration.ssh.host}` : configuration?.address || (machine.id === 'local' ? 'This application' : 'Configured backend route')}</dd></dl>
  {#if machine.state === 'unreachable' || machine.state === 'error'}<p>Existing sessions may still be running here. Reconnect to inspect them before starting replacement work.</p>{/if}
  {#if buildMismatch}<p class="build-warning">Build differs from this Mac ({localBuild}). Sessions are allowed; native workflow plugins may fail to load. Use matching Gimbal builds before running workflows here.</p>{/if}
  <ConnectionChecks progress={connection?.current} />
  {#if connection?.error}<p role="alert">Connection check updates were lost. <button class="action" onclick={() => connection?.reconnect()}>Reconnect updates</button></p>{/if}
  <div class="harness-heading"><h3>Harnesses</h3><button class="text-action" onclick={openTerminal}>{configuration?.ssh?.host || machine.id === 'local' ? 'Open in Terminal' : 'Terminal instructions'}</button></div>
  <p class="inventory-note">This inventory does not check sign-in. A completed session establishes access for that request's model.</p>
  <ul class="harnesses">{#each machine.harnesses as harness (harness.id)}<li><strong>{harness.id}</strong><span class={{ muted: harness.state === 'unknown', trouble: harness.state === 'missing' || harness.state === 'sign-in-required' }}>{harness.state === 'ready' ? 'Ready' : harness.state === 'sign-in-required' ? 'Sign-in required' : harness.state === 'missing' ? 'Not installed' : 'Sign-in not checked'}</span>{#if harness.detail && !(harness.state === 'unknown' && harness.detail === 'Sign-in has not been verified on this machine')}<small>{harness.detail}</small>{/if}{#if harness.state === 'missing' || harness.state === 'sign-in-required'}<small>{harness.state === 'missing' ? `Install ${harness.id}` : `Sign in to ${harness.id}`} on this machine using Terminal.</small>{/if}</li>{:else}<li>Harness availability has not been observed.</li>{/each}</ul>
  <h3>Environments</h3>
  <ul class="environments">{#each environments as row (`${row.environment.project}/${row.environment.id}`)}<li><code>{row.environment.directory}</code><small>{row.directory.git ? `Git · ${row.directory.branch}` : row.directory.unborn ? 'No commits yet' : row.observed ? 'Shared directory' : 'Not inspected'} · {row.live_sessions} sessions</small>{#each row.issues as issue}<small class="trouble">{issue.message}</small>{/each}{#if row.environment.machine !== 'local'}<button class="edit-environment" onclick={() => oneditenvironment(row.environment)}>Edit directory…</button>{/if}</li>{:else}<li>No project directories configured.</li>{/each}</ul>
  <button class="action" onclick={onadd}>Add environment…</button>
  <h3>Capabilities</h3><p>{machine.capabilities.steer ? 'Mid-turn steering' : 'Mid-turn messages queue; steering unavailable'} · {machine.capabilities.fork ? 'Fork supported' : 'Fork unavailable'} · {machine.capabilities.worktree ? 'Worktrees available in Git' : 'Shared checkout only'}</p>
  <h3>Sessions retained here</h3>
  <ul class="sessions">{#each sessions as item (`${item.project}/${item.conversation}`)}<li><a href={`/projects/${item.project}/conversations/${item.conversation}`} target="_blank" rel="noreferrer" onclick={event => openSession(event, `/projects/${item.project}/conversations/${item.conversation}`)}>{item.title || item.conversation}</a><small>{item.status}{item.pending ? ' · outcome or cleanup unconfirmed' : ' · open session'} · {item.directory}</small></li>{:else}<li>No open or uncertain sessions.</li>{/each}</ul>
  <div class="actions"><button class="action" onclick={reconnect} disabled={pending}>{pending ? 'Working…' : 'Reconnect'}</button>{#if machine.id !== 'local'}{#if configuration && configuration.kind !== 'route'}<button class="action" onclick={onedit}>Edit…</button>{/if}<button class="action" onclick={() => { confirm = true; selected = []; error = ''; }}>Remove…</button>{/if}</div>
  {#if error && !confirm}<p role="alert" class="trouble">{error}</p>{/if}
</section>

{#if confirm}
  <Dialog.Root open onOpenChange={(open) => { if (!open) confirm = false; }}><Dialog.Content style="animation: none">
    <Dialog.Header><Dialog.Title>Remove {machine.name}?</Dialog.Title><Dialog.Description>{sessions.length ? 'These sessions still belong to this machine. End only the sessions you select, and wait for confirmed cleanup before removal.' : 'Saved session history will remain readable. This does not stop the remote daemon.'}</Dialog.Description></Dialog.Header>
    <div class="environment-form">{#each sessions as item (`${item.project}/${item.conversation}`)}<label class="radio"><input type="checkbox" bind:group={selected} value={`${item.project}/${item.conversation}`} disabled={pending} />{item.title || item.conversation} · {item.status}{item.pending ? ' · unconfirmed' : ''}</label>{/each}
    {#if sessions.length}<button onclick={endSelected} disabled={pending || !selected.length}>End selected sessions</button>{/if}
    {#if error}<p role="alert" class="error">{error}</p>{/if}<footer><button onclick={() => (confirm = false)} disabled={pending}>Keep machine</button><button class="primary" onclick={remove} disabled={pending || !!sessions.length}>Remove machine</button></footer></div>
  </Dialog.Content></Dialog.Root>
{/if}
{#if terminal}
  <Dialog.Root open onOpenChange={(open) => { if (!open) terminal = false; }}><Dialog.Content style="animation: none"><Dialog.Header><Dialog.Title>Open this machine in a terminal</Dialog.Title><Dialog.Description>{configuration?.ssh?.host ? `Open Terminal and connect to ${configuration.ssh.host} with SSH to diagnose Gimbal or harness sign-in.` : machine.id === 'local' ? 'Open Terminal on this Mac to inspect local harness installation and sign-in.' : 'This connection has no SSH host. Open a terminal on the machine that runs this daemon or backend worker to inspect its harnesses.'}</Dialog.Description></Dialog.Header><button class="action" onclick={() => (terminal = false)}>Done</button></Dialog.Content></Dialog.Root>
{/if}

<style>
  .detail { padding: 24px; overflow: auto; }
  h2 { margin: 0 0 6px; font-size: 19px; font-weight: 600; }
  h3 { margin: 24px 0 8px; font-size: 12px; font-weight: 600; }
  p, li, dl { font-size: 12px; line-height: 1.5; }
  p { color: var(--muted-foreground); }
  .build-warning { color: var(--foreground); }
  .status { margin: 0 0 6px; }
  dl { display: grid; grid-template-columns: 95px 1fr; gap: 7px; margin-top: 18px; }
  dt { color: var(--muted-foreground); } dd { margin: 0; overflow-wrap: anywhere; }
  ul { display: grid; gap: 9px; padding: 0; list-style: none; }
  li { display: grid; gap: 2px; }
  small { color: var(--muted-foreground); font-size: 11px; overflow-wrap: anywhere; }
  .harnesses li { grid-template-columns: 1fr 1.5fr; } .harnesses small { grid-column: 1 / -1; }
  .harness-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
  .inventory-note { margin: 0 0 10px; }
  .muted { color: var(--muted-foreground); }
  .text-action { border: 0; padding: 0; background: none; color: inherit; font: inherit; font-size: 12px; text-decoration: underline; cursor: pointer; }
  code { overflow-wrap: anywhere; font-size: 12px; }
  .edit-environment { justify-self: start; border: 0; padding: 0; background: none; color: inherit; font: inherit; text-decoration: underline; cursor: pointer; }
  .actions { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 24px; }
  .trouble { color: var(--destructive); }
  a { color: inherit; text-decoration: underline; text-underline-offset: 2px; }
</style>
