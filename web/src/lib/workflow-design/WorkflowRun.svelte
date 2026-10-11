<script lang="ts">
 import {launchWorkflow} from '../../routes/launch.remote';
 import type {RunField,RunRole,RunProject} from '../../routes/workflows/[workflow]/types';
 type WorkflowData={name:string;help:string;fields:RunField[];roles:RunRole[];projects:RunProject[]};
 let {workflow}:{workflow:WorkflowData}=$props();
 const form=launchWorkflow;
</script>
<section class="run-workflow" aria-label="Run workflow">
 <h2>Run {workflow.name}</h2>
 <p>{workflow.help}</p>
 <form {...form}>
  <input {...form.fields.workflow.as("hidden",workflow.name)}/>
  <label>Project <select {...form.fields.project.as("select", workflow.projects[0]?.path ?? "")} required>{#each workflow.projects as project}<option value={project.path}>{project.path}</option>{/each}</select></label>
  <label>Work directory <input {...form.fields.workdir.as("text")} placeholder="Project directory when empty"/></label>
  {#each workflow.fields as field,i}
   <fieldset>
    <legend>{field.name}</legend>
    <input {...form.fields.inputs[i].name.as("hidden",field.name)}/>
    {#if field.optional}<label class="include"><input {...form.fields.inputs[i].include.as("checkbox")}/> Include this value</label>
    {:else}<input {...form.fields.inputs[i].include.as("hidden",true)}/>{/if}
    <label for={`input-${i}`}>{field.description || field.name}</label>
    {#if field.kind==='bool'}
     <select id={`input-${i}`} {...form.fields.inputs[i].value.as(field.kind==="bool"?"select":"text")}><option value="false">False</option><option value="true">True</option>{#if field.nullable}<option value="null">Null</option>{/if}</select>
    {:else}<input id={`input-${i}`} {...form.fields.inputs[i].value.as(field.kind==="bool"?"select":"text")} inputmode={field.kind==='int'?'numeric':undefined} placeholder={field.nullable?'Value, or null':undefined}/>{/if}
    {#each form.fields.inputs[i].value.issues() ?? [] as issue}<p class="error" role="alert">{issue.message}</p>{/each}
   </fieldset>
  {/each}
  {#if workflow.roles.length}<fieldset><legend>Models</legend>
   {#each workflow.roles as role,i}<input {...form.fields.roles[i].name.as("hidden",role.name)}/><label>{role.name}<input {...form.fields.roles[i].value.as("text",role.default)}/></label>{/each}
  </fieldset>{/if}
  {#each form.fields.allIssues() ?? [] as issue}<p class="error" role="alert">{issue.message}</p>{/each}
  <button type="submit" disabled={form.pending>0}>{form.pending?'Starting…':'Run workflow'}</button>
  {#if form.result}<p role="status">Run started. <a href={`/projects/${form.result.project_id}/runs/${form.result.run_id}`}>Open run</a></p>{/if}
 </form>
</section>
<style>
 .run-workflow{padding:20px 24px;max-width:760px}h2{font-size:20px;margin:0 0 8px}form{display:grid;gap:16px}label{display:grid;gap:5px;font-size:14px}input,select{border:1px solid #8d959c;border-radius:4px;padding:8px;background:white;color:#17232b}fieldset{display:grid;gap:8px;border:1px solid #b1b4b6;border-radius:4px;padding:12px}legend{font-weight:600}.include{display:flex;align-items:center;gap:8px}button{justify-self:start;background:#165a91;color:white;border:0;border-radius:4px;padding:10px 16px;cursor:pointer}.error{color:#a32620;font-size:14px;margin:0}
</style>
