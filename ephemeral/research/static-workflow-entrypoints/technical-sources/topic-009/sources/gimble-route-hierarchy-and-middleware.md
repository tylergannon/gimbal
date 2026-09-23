# Gimble Route Hierarchy, Navigation Layout, and Project Scoping Middleware

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble`
- **Files**:
  - `web/runtime.go`
  - `web/src/routes/+layout.svelte`
  - `web/src/routes/+page.svelte`
  - `web/src/routes/projects/[project]/+page.svelte`
  - `web/src/project.go`
- **Commit/Baseline**: `main` / `e161721c`
- **Retrieval Date**: 2026-09-23

---

## 1. Project Scoping Middleware in `web/runtime.go`

From `web/runtime.go` (lines 366–404):

```go
func (i *Instance) projectRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/_app/") {
			// Kit's remote endpoint is shared by all pages. The browser supplies
			// its project path in Referer for each tab's request.
			path = r.Header.Get("Referer")
			if parsed, err := url.Parse(path); err == nil {
				path = parsed.Path
			}
		}
		var p *Runtime
		i.mu.RLock()
		choices := make([]hooks.ProjectChoice, 0, len(i.projects))
		for _, candidate := range i.projects {
			choices = append(choices, hooks.ProjectChoice{ID: candidate.id, Path: candidate.project})
			if strings.HasPrefix(path, "/projects/"+candidate.id) &&
				(len(path) == len("/projects/")+len(candidate.id) || path[len("/projects/")+len(candidate.id)] == '/') {
				p = candidate
				break
			}
		}
		i.mu.RUnlock()
		slices.SortFunc(choices, func(a, b hooks.ProjectChoice) int { return strings.Compare(a.Path, b.Path) })
		if p == nil {
			if strings.HasPrefix(r.URL.Path, "/projects/") || strings.Contains(r.URL.Path, "/remote/") || strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(hooks.WithProjects(r.Context(), choices)))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/projects/"+p.id+"/api/") {
			r = r.Clone(r.Context())
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/projects/"+p.id)
		}
		next.ServeHTTP(w, r.WithContext(p.requestContext(r.Context())))
	})
}
```

---

## 2. Navigation Layout and Route Hierarchy

From `web/src/routes/+layout.svelte` (lines 7–21):

```svelte
<script lang="ts">
	import '../app.css';
	import { navigating, page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';
	import RunsLoading from '#lib/run/RunsLoading.svelte';

	let { children } = $props();
	const project = $derived(page.params.project);
	const root = $derived(project ? `/projects/${project}` : '/');
	const loadingRuns = $derived(navigating.to?.url.pathname === root && navigating.from?.url.pathname !== root);
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
<nav class="site-nav"><a href="/">Projects</a>{#if project}<a href={`${root}/conversations`}>Conversations</a><a href={root}>Runs</a><a href={`${root}/about`}>About</a>{/if}</nav>
<main>
	{#if loadingRuns}
		<div class="runs-page"><RunsLoading /></div>
	{:else}
		{@render children()}
	{/if}
</main>
```

---

## 3. Instance Root Projects Page

From `web/src/routes/+page.svelte` (lines 1–14):

```svelte
<script lang="ts">
  let { data }: { data: { projects: { id: string; path: string }[] } } = $props();
</script>

<svelte:head><title>Projects — Gimble</title></svelte:head>
<section class="projects">
  <h1>Projects</h1>
  {#each data.projects as project (project.id)}
    <a href={`/projects/${project.id}`}>{project.path}</a>
  {:else}
    <p>No projects admitted.</p>
  {/each}
</section>
```

---

## 4. Context Projection Types in `web/src/project.go`

From `web/src/project.go` (lines 8–20):

```go
type ProjectChoice struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func WithProjects(ctx context.Context, choices []ProjectChoice) context.Context {
	return context.WithValue(ctx, projectsKey{}, choices)
}

func Projects(ctx context.Context) []ProjectChoice {
	choices, _ := ctx.Value(projectsKey{}).([]ProjectChoice)
	return choices
}
```
