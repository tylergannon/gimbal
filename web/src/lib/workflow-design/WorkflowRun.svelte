<script lang="ts">
  import { launchWorkflow } from '../../routes/launch.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import type { RunField, RunRole, RunProject } from '../../routes/workflows/[workflow]/types';
  type WorkflowData = { name: string; help: string; summary?: string; fields: RunField[]; roles: RunRole[]; projects?: RunProject[] };
  let { workflow, project, onstarted, onguide }: {
    workflow: WorkflowData; project?: string;
    onstarted?: (accepted: { project_id: string; run_id: string }) => void;
    onguide?: () => void;
  } = $props();
  const instance = $props.id();
  const form = $derived(launchWorkflow.for(`launch-${project ?? 'catalog'}-${workflow.name}-${instance}`));
  let actionError = $state('');
</script>

<section class="launch-form" aria-label="Run workflow">
  <p class="purpose">{workflow.summary || workflow.help}</p>
  <form {...form.enhance(async (submitted) => {
    actionError = '';
    try { await submitted.submit(); if (submitted.result) onstarted?.(submitted.result); }
    catch (error) { if (!navigateOnUnauthorized(error)) actionError = errorMessage(error); }
  })}>
    <input {...form.fields.workflow.as('hidden', workflow.name)} />
    {#if project}
      <input {...form.fields.project.as('hidden', project)} />
      <p class="project" title={project}>In <strong>{project.split('/').filter(Boolean).at(-1)}</strong></p>
    {:else}
      <label>Project<select {...form.fields.project.as('select', workflow.projects?.[0]?.path ?? '')} required>
        {#each workflow.projects ?? [] as item (item.path)}<option value={item.path}>{item.path}</option>{/each}
      </select></label>
    {/if}
    {#each workflow.fields as field, i (field.name)}
      <div class="field">
        <input {...form.fields.inputs[i].name.as('hidden', field.name)} />
        {#if field.optional}<label class="include"><input {...form.fields.inputs[i].include.as('checkbox')} /> Include {field.name}</label>
        {:else}<input {...form.fields.inputs[i].include.as('hidden', true)} />{/if}
        <label for={`workflow-${workflow.name}-${i}`}>{field.name.toLowerCase() === 'goal' ? 'Goal' : field.name}</label>
        {#if field.description}<p class="description">{field.description}</p>{/if}
        {#if field.kind === 'bool'}
          <select id={`workflow-${workflow.name}-${i}`} {...form.fields.inputs[i].value.as('select')}><option value="false">False</option><option value="true">True</option>{#if field.nullable}<option value="null">Null</option>{/if}</select>
        {:else if field.kind === 'string'}
          <textarea id={`workflow-${workflow.name}-${i}`} {...form.fields.inputs[i].value.as('text')} rows={field.name.toLowerCase() === 'goal' ? 2 : 1} placeholder={field.nullable ? 'Value, or null' : undefined}></textarea>
        {:else}
          <input id={`workflow-${workflow.name}-${i}`} {...form.fields.inputs[i].value.as('text')} inputmode="numeric" placeholder={field.nullable ? 'Value, or null' : undefined} />
        {/if}
        {#each form.fields.inputs[i].value.issues() ?? [] as issue (issue.message)}<p class="error" role="alert">{issue.message}</p>{/each}
      </div>
    {/each}
    {#if workflow.roles.length}
      <details class="overrides"><summary>Model override <span>optional</span></summary>
        {#each workflow.roles as role, i (role.name)}
          <input {...form.fields.roles[i].name.as('hidden', role.name)} />
          <label>{role.name}<input {...form.fields.roles[i].value.as('text', role.default)} /></label>
        {/each}
      </details>
    {/if}
    <details class="overrides"><summary>Work directory <span>optional</span></summary><label>Directory<input {...form.fields.workdir.as('text')} placeholder="Project directory when empty" /></label></details>
    <p class="typical">Typical: {workflow.roles.length} agent {workflow.roles.length === 1 ? 'role' : 'roles'}; cost depends on turns and the selected models.</p>
    {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p class="error" role="alert">{issue.message}</p>{/each}
    {#if actionError}<p class="error" role="alert">{actionError}</p>{/if}
    <div class="actions">
      {#if onguide}<button type="button" class="guide" onclick={onguide}>Read the guide</button>{/if}
      <button class="start" type="submit" disabled={!!form.pending}>{form.pending ? 'Starting…' : 'Start run'}</button>
    </div>
    {#if form.result && !onstarted}<p role="status">Run started. <a href={`/projects/${form.result.project_id}/runs/${form.result.run_id}`}>Open run</a></p>{/if}
  </form>
</section>
<style>
  .launch-form, form, .field { display: grid; gap: 12px; min-width: 0; }
  .purpose { margin: 0; font-size: 13px; line-height: 1.6; color: var(--muted-foreground); }
  label { display: grid; gap: 6px; font-size: 12px; font-weight: 500; }
  .field { gap: 5px; }
  .include { display: flex; align-items: center; }
  input:not([type='hidden']):not([type='checkbox']), select, textarea { box-sizing: border-box; width: 100%; min-width: 0; padding: 8px 10px; border: 1px solid var(--border); border-radius: 7px; background: var(--background); color: var(--foreground); font: inherit; font-size: 13px; }
  textarea { resize: vertical; min-height: 38px; }
  input:focus-visible, select:focus-visible, textarea:focus-visible, button:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
  .project, .description, .typical { margin: 0; color: var(--muted-foreground); font-size: 11px; line-height: 1.5; }
  .project strong { color: var(--foreground); font-weight: 500; }
  .overrides summary { cursor: pointer; font-size: 12px; }
  .overrides summary span { margin-left: 4px; color: var(--muted-foreground); font-size: 11px; }
  .overrides label { margin-top: 10px; }
  .actions { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
  button { cursor: pointer; font: inherit; font-size: 12px; }
  button:disabled { opacity: .45; cursor: default; }
  .guide { padding: 5px 0; background: transparent; border: 0; color: var(--muted-foreground); text-decoration: underline; text-underline-offset: 3px; }
  .start { margin-left: auto; padding: 7px 13px; border: 0; border-radius: 7px; background: var(--foreground); color: var(--background); font-weight: 500; }
  .error { margin: 0; font-size: 12px; color: var(--destructive); }
</style>
