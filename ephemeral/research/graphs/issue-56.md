# Per-node skills: declare skills on a workflow step, resolved and scoped by the engine

URL: https://github.com/tylergannon/gimble/issues/56
State: closed
Milestone: None
Updated: 2026-09-13T16:47:04Z

## Summary

Let a workflow step declare the skills it needs, by name, path, or GitHub URL. The engine resolves them, pins them, and puts them in scope for that node only. Some skills ship inside the binary; the rest are downloaded and cached.

```yaml
- id: implement
  type: agent
  skills:
    - tractor:sprint-execute
    - ./team-skills/house-style
    - github.com/diffusioninc/skills@d0cdcd6#.claude/skills/df-sprint-plan
  prompt: $goal
```

## Why

Two reasons, and the second is the urgent one.

**Doctrine belongs in skills, not in prompts.** The workflows worth building — a sprint loop, a chapter loop, a promise loop — are small graphs wrapped around a lot of prose about how the work is done. Today that prose has to be pasted into node prompts, which makes it unversionable, unshareable, and impossible to reuse across pipelines. As a skill it travels on its own, and the graph goes back to being control flow.

**Skills already leak into runs, undeclared.** Tractor never sets `SkillsConfig`, so the Claude SDK defaults apply (`EnableSkills: true`, `SettingSources: ["user", "project"]`) and the transport emits `--setting-sources user,project`. Every Claude node in every run today loads whatever is installed in the operator's `~/.claude/skills` and in the workdir's `.claude/skills`. Nothing about that appears in the pipeline file, nothing is pinned, and two machines can run the same graph differently. A pipeline is supposed to be the whole story of a run. Declaring skills per node is also how that gets closed: a node with no `skills` field should load none.

## Mechanisms

Both real harnesses already have a per-turn hook.

**Claude.** The CLI accepts `--plugin-dir` and `--plugin-dir-no-mcp`; a plugin directory is a directory with `skills/` in it. The adapter already passes arbitrary flags through `WithExtraArgs` (`harness/claude/native.go`, used for `json-schema` and `session-id`), and opens a fresh client per turn (`harness/claude/adapter.go`, `runNativeTurn`), so `nativeConfig` is already the per-turn place to put this. No SDK change needed. Prefer `--plugin-dir-no-mcp` so a fetched bundle cannot introduce MCP servers.

**Codex.** The app-server turn input has a native skill element: `SkillUserInput {type: "skill", name, path}` in `harness/codex/schema/TurnStartParams.json`. Turn input is built in `nativeInput` (`harness/codex/adapter.go`), which today emits text parts only. Attaching a skill to a single turn is a change to that one function.

**agy.** No skill surface; the adapter passes `-p`, `--add-dir`, model, effort, and schema.

So the portable floor is a frame. `engine/frames.go` already injects engine-rendered blocks ahead of codergen and fan-in prompts. A skills frame listing each skill's name, description, and path — body left on disk for the agent to read — works on every harness including agy, and is the same progressive disclosure the native mechanisms use. Native attachment is then a per-harness optimization, not a prerequisite.

## Shape

Add `skills` to `LLMNodeFields` so agent, fan-in, and loop roles all get it, and to pipeline `defaults`.

Three source forms:

- `tractor:<name>` — bundled in the binary via `embed.FS`
- a local path — relative to the pipeline file
- `<host>/<owner>/<repo>@<ref>#<subdir>` — fetched with a shallow clone, `<subdir>` optional

One resolver in `internal/skills`: resolve, cache under a shared root keyed by resolved commit, materialize a per-node directory in the run directory by symlink. All three forms land in the same cache so one code path materializes them.

Resolution happens during the `start_run` lint pass. An unknown skill name, a missing path, an unreachable repository, or a missing subdirectory fails the run before a token burns, the same as any other graph error.

## Pinning

A ref resolves to a commit at lint time and the commit goes in the run manifest. An unpinned source should be a lint error, not a warning. These bundles ship executable scripts, and the agent that reads them runs with permissions bypassed, so an unpinned URL is remote code execution against the operator's machine. The resolved commit is also what makes a run reproducible.

## Acceptance criteria

1. `skills` is a typed, optional field on agent, fan-in, and loop-role nodes, and on pipeline defaults. Unknown fields inside it are parse errors like everywhere else.
2. The three source forms resolve: bundled, local path, and repository URL with optional subdirectory and required ref.
3. Resolution and materialization happen at lint time; every failure mode above stops the run before it starts.
4. A resolved skill set is recorded in the run manifest with the commit each source resolved to.
5. A Claude node receives its skills through a per-node plugin directory and no others; a node with no `skills` field loads no skills, including none from `~/.claude/skills` or the workdir.
6. A Codex node receives its skills as skill inputs on its own turns.
7. An agy node, and any node whose harness has no native mechanism, receives a skills frame naming each skill and its path.
8. Skills declared on a loop role apply on every lap without re-fetching.
9. At least one skill ships in the binary and is reachable as `tractor:<name>` with no network.

## Slices

1. Bundled skills, local paths, the frame fallback, and the Claude plugin directory. Useful on its own and needs no network.
2. The repository fetcher: resolver, cache, pinning, lint-time failures, manifest record.
3. Codex native skill inputs.

## Note

The upstream bundles this is modelled on keep two byte-mirrored copies of every skill, one for Claude and one for other runtimes, with a validation script enforcing parity. That duplication exists only because there is no common runtime. If the engine materializes per-node skill directories itself, it reads one bundle and presents it in whichever shape each harness wants, and the mirror problem does not reach anyone running through Tractor.

