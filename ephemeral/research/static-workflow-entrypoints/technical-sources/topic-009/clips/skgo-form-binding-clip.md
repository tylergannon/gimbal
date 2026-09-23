# Clip: Idiomatic SvelteKit 5 Form Remote Binding Pattern for Workflow Start

This clip shows the concrete, idiomatic pattern for mounting a visible start form using SvelteKit 5 / SKGO `form` remotes and `shadcn-svelte` primitives, complying with Tyler's hard rules in `ephemeral/research/svelte-idioms/dos-and-donts.md`:

```svelte
<script lang="ts">
  import { goto } from "$app/navigation";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import { Card, CardHeader, CardTitle, CardContent, CardFooter } from "#lib/components/ui/card/index.js";
  import { startReview } from "../start-review.remote.js";

  let { data }: { data: { projects: { id: string; path: string }[] } } = $props();

  const form = startReview;
  let serverError = $state("");

  form.enhance(async ({ submit }) => {
    serverError = "";
    try {
      const ok = await submit();
      if (ok && form.result) {
        // Direct route navigation to the newly admitted run page
        await goto(`/projects/${form.result.project_id}/runs/${encodeURIComponent(form.result.run_id)}`);
      }
    } catch (err) {
      serverError = err instanceof Error ? err.message : String(err);
    }
  });
</script>

<Card class="max-w-xl mx-auto my-8">
  <CardHeader>
    <CardTitle>Start Code Review</CardTitle>
  </CardHeader>
  <CardContent>
    <!-- Spreading form attaches SvelteKit's event handlers and attributes -->
    <form {...form} class="space-y-4">
      <!-- Project directory input -->
      <div class="space-y-1">
        <label for="project-dir" class="text-sm font-medium">Project Directory</label>
        <Input
          id="project-dir"
          placeholder="/path/to/project"
          {...form.fields.project_dir.as('text')}
        />
        {#each form.fields.project_dir.issues() ?? [] as issue}
          <p class="text-xs text-destructive">{issue.message}</p>
        {/each}
      </div>

      <!-- Workflow target parameter -->
      <div class="space-y-1">
        <label for="target" class="text-sm font-medium">Review Target (Commit or PR)</label>
        <Input
          id="target"
          placeholder="HEAD~1"
          {...form.fields.target.as('text')}
        />
        {#each form.fields.target.issues() ?? [] as issue}
          <p class="text-xs text-destructive">{issue.message}</p>
        {/each}
      </div>

      <!-- Server failure envelope display -->
      {#if serverError}
        <div class="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {serverError}
        </div>
      {/if}

      <!-- Submit button with pending state -->
      <CardFooter class="px-0 pt-4">
        <Button type="submit" disabled={Boolean(form.pending)}>
          {#if form.pending}
            Starting Review...
          {:else}
            Start Review
          {/if}
        </Button>
      </CardFooter>
    </form>
  </CardContent>
</Card>
```
