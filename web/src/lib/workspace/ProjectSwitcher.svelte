<script lang="ts">
  import { goto, refreshAll } from '$app/navigation';
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
  import Folder from '@lucide/svelte/icons/folder';
  import * as Dropdown from '#lib/components/ui/dropdown-menu';
  import * as Dialog from '#lib/components/ui/dialog';
  import { createProject } from '#lib/workspace.remote';
  import { openProject } from '#lib/window.remote';
  import { chooseProjectFolder, nativeMessage } from './native';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import type { ProjectChoice } from '#lib/skgo/host/types';
  import ProjectOption from './ProjectOption.svelte';
  let { project, name, path, projects, native, compact = false, open = $bindable(false), onselect }: { project:string; name:string; path:string; projects:ProjectChoice[]; native:boolean; compact?:boolean; open?:boolean; onselect:(path:string)=>void } = $props();
  let folderDialog = $state(false);
  let feedback = $state('');
  const form = createProject.for('project-switcher');
  async function openFolder() {
    open = false;
    if (!native) { folderDialog = true; return; }
    const chosen = await chooseProjectFolder(); if (!chosen) return;
    try { const admitted = await openProject({path:chosen}); await refreshAll(); if (window.webkit?.messageHandlers?.requestProject) nativeMessage('requestProject',{path:`/projects/${admitted.id}`,projectPath:chosen}); else await goto(`/projects/${admitted.id}`,{invalidateAll:true}); }
    catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); }
  }
</script>
<Dropdown.Root bind:open><Dropdown.Trigger class={['project-chip',{compact}]} aria-label={`Switch project, ${name}`}><span class="tile">{name.slice(0,1).toLowerCase()}</span><span class="project-copy"><b>{name}</b>{#if !compact}<span title={path}>{path}</span>{/if}</span><ChevronsUpDown size={12} /></Dropdown.Trigger><Dropdown.Content align="start" side="bottom" sideOffset={6} class="w-[340px] max-w-[90vw]"><Dropdown.Label>Projects</Dropdown.Label>{#each projects as choice (choice.id)}<ProjectOption {choice} current={project} onchoose={id => { open = false; onselect(`/projects/${id}`); }} />{/each}<Dropdown.Separator /><Dropdown.Item onclick={openFolder}><Folder size={14} />Open folder…</Dropdown.Item></Dropdown.Content></Dropdown.Root>
{#if feedback}<p role="alert">{feedback}</p>{/if}
<Dialog.Root bind:open={folderDialog}><Dialog.Content><Dialog.Header><Dialog.Title>Open project folder</Dialog.Title><Dialog.Description>Choose an existing directory to open in Gimbal.</Dialog.Description></Dialog.Header><form {...form.enhance(async submitted => { feedback = ''; try { if (await submitted.submit() && submitted.result) { folderDialog = false; await goto(`/projects/${submitted.result.id}`,{invalidateAll:true}); } } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); } })}><label>Project directory<input {...form.fields.path.as('text')} placeholder="/absolute/path/to/project" required /></label><button disabled={!!form.pending}>{form.pending ? 'Opening…' : 'Open project'}</button>{#each form.fields.allIssues() ?? [] as issue (issue.message)}<p role="alert">{issue.message}</p>{/each}{#if feedback}<p role="alert">{feedback}</p>{/if}</form></Dialog.Content></Dialog.Root>
<style>
  :global(.project-chip.compact) { width:auto; max-width:150px; flex:none; }
  :global(.project-chip) { display:flex; align-items:center; gap:8px; width:100%; min-width:0; padding:8px 10px; color:var(--sidebar-foreground); border:0; border-radius:6px; background:transparent; text-align:left; cursor:pointer; } :global(.project-chip:hover) { background:var(--sidebar-accent); } .tile { display:grid; place-items:center; flex:none; width:24px; height:24px; border-radius:50%; background:var(--foreground); color:var(--background); font:600 14px var(--font-sans); } .project-copy { display:flex; flex-direction:column; gap:1px; min-width:0; flex:1; } b { font:600 13px/18px var(--font-sans); } .project-copy>span { color:var(--muted-foreground); font:10px/14px var(--font-mono); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } form { display:grid; gap:12px; } label { display:grid; gap:6px; font-size:13px; } input { padding:8px; border:1px solid var(--border); border-radius:6px; color:var(--foreground); background:var(--background); font:13px var(--font-mono); } form button { justify-self:end; padding:7px 12px; border:0; border-radius:6px; background:var(--foreground); color:var(--background); cursor:pointer; } p { font-size:12px; color:var(--destructive); }
</style>
