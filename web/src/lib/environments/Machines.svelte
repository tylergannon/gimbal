<script lang="ts">
  import { untrack } from 'svelte';
  import { watchEnvironments, environmentConfig } from '#lib/environments.remote';
  import { errorMessage } from '#lib/errors';
  import type { EnvironmentInfo } from '#lib/skgo/environments/types';
  import { machineConfigurations, type MachineConfiguration } from './configuration';
  import MachineDetail from './MachineDetail.svelte';
  import MachineConnect from './MachineConnect.svelte';
  import AddEnvironment from './AddEnvironment.svelte';
  import ConfigEditor from './ConfigEditor.svelte';
  import './environments.css';

  let { project = '', projects = [] }: { project?: string; projects?: { id: string; path: string }[] } = $props();
  const stream = watchEnvironments('');
  const configuration = environmentConfig();
  const snapshot = $derived(stream.current);
  const machines = $derived(snapshot?.machines ?? []);
  let selected = $state('local');
  const machine = $derived(machines.find(item => item.id === selected) ?? machines[0]);
  const configurations = $derived(machineConfigurations(configuration.current?.text ?? ''));
  const selectedConfig = $derived(configurations.find(item => item.id === machine?.id));
  let connecting = $state(false);
  let editing = $state<MachineConfiguration>();
  let adding = $state(untrack(() => !!project));
  let forProject = $state(untrack(() => project));
  let editingEnvironment = $state<EnvironmentInfo>();
  let editor = $state(false);
  function changed() { stream.reconnect(); void configuration.refresh(); }
</script>

<section class="machine-settings" aria-label="Machines settings">
  <header><div><h1>Machines</h1><p>Where your project directories and agent sessions live.</p></div><div class="tools"><button class="action" onclick={() => (editor = true)}>Open configuration</button><button class="action" onclick={() => { connecting = true; editing = undefined; }}>Add machine…</button></div></header>
  {#if snapshot}
    {#each snapshot.issues as issue}<p class="issue" role="alert">{issue.line ? `Line ${issue.line}: ` : ''}{issue.entry ? `${issue.entry}: ` : ''}{issue.message}</p>{/each}
    <div class="body"><nav aria-label="Configured machines">{#each machines as item (item.id)}<button class:selected={machine?.id === item.id} aria-current={machine?.id === item.id ? 'true' : undefined} onclick={() => (selected = item.id)}><strong>{item.name}</strong><span>{item.id === 'local' ? 'This application' : item.kind === 'ssh' ? 'SSH tunnel' : item.kind === 'route' ? 'Backend route' : 'HTTP daemon'}</span><small class:trouble={item.state === 'error' || item.state === 'unreachable'}>{item.state === 'connected' ? `Connected · ${item.latency_ms} ms` : item.state === 'not-started' ? 'Starts when needed' : item.state}</small></button>{/each}</nav>
      {#if machine}{#key machine.id}<MachineDetail {machine} configuration={selectedConfig} {snapshot} onchanged={changed} onedit={() => { editing = selectedConfig; connecting = true; }} onadd={() => { editingEnvironment = undefined; adding = true; }} oneditenvironment={(environment) => { editingEnvironment = environment; forProject = environment.project; adding = true; }} />{/key}{/if}
    </div>
    <footer>Saved in <code>{snapshot.config_path}</code>. File edits apply automatically. {#if !stream.connected}<span role="status">Connection to the app server was lost. <button onclick={() => stream.reconnect()}>Reconnect</button></span>{/if}</footer>
    {#if connecting}<MachineConnect revision={snapshot.revision} machine={editing} onclose={() => (connecting = false)} onsaved={(id) => { selected = id; connecting = false; changed(); }} onconfig={() => { connecting = false; editor = true; }} />{/if}
    {#if adding}
      {#if forProject}<AddEnvironment {snapshot} project={forProject} environment={editingEnvironment} machineID={machine?.id} onclose={() => (adding = false)} onsaved={() => { adding = false; changed(); }} />
      {:else}<div class="choose-project"><label>Project<select bind:value={forProject}><option value="">Choose a project</option>{#each projects as item (item.id)}<option value={item.id}>{item.path}</option>{/each}</select></label>{#if !projects.length}<p>Open a project directory in the main window before adding an environment.</p>{/if}<button class="action" onclick={() => (adding = false)}>Cancel</button></div>{/if}
    {/if}
  {:else if stream.error}<p class="issue" role="alert">{errorMessage(stream.error)} <button onclick={() => stream.reconnect()}>Retry</button></p>{:else}<p class="loading" role="status">Loading machines…</p>{/if}
  {#if editor}<ConfigEditor onclose={() => (editor = false)} onsaved={() => { editor = false; changed(); }} />{/if}
</section>

<style>
  .machine-settings { display: flex; flex-direction: column; height: 100vh; min-height: 500px; color: var(--foreground); background: var(--background); }
  header { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; padding: 20px 24px; border-bottom: 1px solid var(--border); }
  h1 { margin: 0; font-size: 18px; font-weight: 600; } header p { margin: 4px 0 0; font-size: 12px; color: var(--muted-foreground); }
  .tools { display: flex; gap: 7px; }
  .body { display: grid; grid-template-columns: 225px minmax(0, 1fr); flex: 1; min-height: 0; }
  nav { padding: 10px; overflow: auto; border-right: 1px solid var(--border); background: var(--sidebar); }
  nav button { display: grid; gap: 3px; width: 100%; padding: 12px; margin-bottom: 4px; border: 0; border-radius: 8px; background: none; color: inherit; text-align: left; font: inherit; font-size: 13px; cursor: pointer; }
  nav button.selected, nav button:hover { background: var(--sidebar-accent); } nav strong { font-weight: 500; } nav span, nav small { color: var(--muted-foreground); font-size: 11px; }
  footer { padding: 11px 24px; border-top: 1px solid var(--border); color: var(--muted-foreground); font-size: 11px; overflow-wrap: anywhere; }
  footer span { display: block; margin-top: 4px; } footer button { color: inherit; }
  code { font-size: 11px; }
  .issue { margin: 10px 24px; color: var(--destructive); font-size: 12px; } .trouble { color: var(--destructive); }
  .loading { margin: 24px; color: var(--muted-foreground); font-size: 13px; }
  .choose-project { position: fixed; inset: 25% 15%; z-index: 60; padding: 24px; border: 1px solid var(--border); border-radius: 12px; background: var(--background); box-shadow: var(--shadow-lg); }
  .choose-project label { display: grid; gap: 10px; margin-bottom: 16px; } .choose-project p { font-size: 13px; }
  @media (max-width: 650px) { .body { grid-template-columns: 175px minmax(0, 1fr); } }
</style>
