<script lang="ts">
  import { launchWorkflow } from "../../routes/launch.remote";
  import type { RunField, RunRole, RunProject } from "../../routes/workflows/[workflow]/types";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  type WorkflowData = {
    name: string;
    help: string;
    fields: RunField[];
    roles: RunRole[];
    projects: RunProject[];
  };
  let { workflow }: { workflow: WorkflowData } = $props();
  const form = launchWorkflow;
  // A native select, drawn like the Input primitive, so the form field binds to it.
  const select =
    "h-9 w-full rounded-md border border-input bg-transparent px-2.5 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";
</script>

<section class="grid max-w-3xl gap-2 py-4 text-sm" aria-label="Run workflow">
  <h2 class="text-lg font-semibold">Run {workflow.name}</h2>
  <p class="text-muted-foreground">{workflow.help}</p>
  <form class="grid gap-4" {...form}>
    <input {...form.fields.workflow.as("hidden", workflow.name)} />
    <label class="grid gap-1.5 font-medium"
      >Project <select
        class={select}
        {...form.fields.project.as("select", workflow.projects[0]?.path ?? "")}
        required
        >{#each workflow.projects as project (project.path)}<option value={project.path}
            >{project.path}</option
          >{/each}</select
      ></label
    >
    <label class="grid gap-1.5 font-medium"
      >Work directory <Input
        {...form.fields.workdir.as("text")}
        placeholder="Project directory when empty"
      /></label
    >
    {#each workflow.fields as field, i (field.name)}
      <fieldset class="grid gap-2 rounded-lg border p-3">
        <legend class="px-1 font-medium">{field.name}</legend>
        <input {...form.fields.inputs[i].name.as("hidden", field.name)} />
        {#if field.optional}<label class="flex items-center gap-2"
            ><input {...form.fields.inputs[i].include.as("checkbox")} /> Include this value</label
          >
        {:else}<input {...form.fields.inputs[i].include.as("hidden", true)} />{/if}
        <label class="text-muted-foreground" for={`input-${i}`}
          >{field.description || field.name}</label
        >
        {#if field.kind === "bool"}
          <select class={select} id={`input-${i}`} {...form.fields.inputs[i].value.as("select")}
            ><option value="false">False</option><option value="true">True</option
            >{#if field.nullable}<option value="null">Null</option>{/if}</select
          >
        {:else}<Input
            id={`input-${i}`}
            {...form.fields.inputs[i].value.as("text")}
            inputmode={field.kind === "int" ? "numeric" : undefined}
            placeholder={field.nullable ? "Value, or null" : undefined}
          />{/if}
        {#each form.fields.inputs[i].value.issues() ?? [] as issue}<p
            class="text-destructive"
            role="alert"
          >
            {issue.message}
          </p>{/each}
      </fieldset>
    {/each}
    {#if workflow.roles.length}<fieldset class="grid gap-2 rounded-lg border p-3">
        <legend class="px-1 font-medium">Models</legend>
        {#each workflow.roles as role, i (role.name)}<input
            {...form.fields.roles[i].name.as("hidden", role.name)}
          /><label class="grid gap-1.5"
            >{role.name}<Input {...form.fields.roles[i].value.as("text", role.default)} /></label
          >{/each}
      </fieldset>{/if}
    {#each form.fields.allIssues() ?? [] as issue}<p class="text-destructive" role="alert">
        {issue.message}
      </p>{/each}
    <Button class="justify-self-start" type="submit" disabled={form.pending > 0}
      >{form.pending ? "Starting…" : "Run workflow"}</Button
    >
    {#if form.result}<p role="status">
        Run started. <a
          class="font-medium underline underline-offset-4"
          href={`/projects/${form.result.project_id}/runs/${form.result.run_id}`}>Open run</a
        >
      </p>{/if}
  </form>
</section>
