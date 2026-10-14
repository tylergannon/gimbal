# Model configuration

Resolve models before substantive work or any model preflight. Defaults live in
this skill's `SKILL.md`: Easy Loop uses the `agents` entries in its workflow;
other skills use `model_defaults`. Overlay these files in order, field by field:

1. `~/.config/diffusion/skills/models.yaml`
2. `<project-dir>/.diffusion/skills/models.yaml`

The project file wins; omitted roles and fields retain their lower-layer values.
Missing, blank, or comment-only files mean no overrides. An empty mapping
(`{}`) is also valid. Malformed
YAML, duplicate keys, unknown roles for this skill, and invalid model/effort
values are errors; report them before launching. Entries for other skills are
ignored. `reasoning_effort: null` explicitly removes an inherited effort flag.

Both files use the same schema (include only the overrides you want):

```yaml
skills:
  df-easy-loop-e2e:
    coding:
      model: gpt-6-astra
      reasoning_effort: high
  df-easy-loop-simple:
    plan:
      model: claude-fable-5-1
  df-sprint-plan:
    claude:
      model: claude-fable-5-1
  df-sprint-execute:
    codex:
      model: gpt-6-astra
  df-chapter-create:
    main:
      model: claude-fable-5-1
  df-semantic-index:
    reader:
      model: gpt-6-astra
  df-promise:
    main:
      model: gpt-6-astra
```

Use the bundled `scripts/resolve_models` Go module beside the **loaded**
SKILL.md; do not search for a different installation. It includes its pinned
YAML dependency and requires the normal Go toolchain, independent of the target
project's module dependencies.

Substitute the actual installed skill directory and absolute target project root:

```text
GOWORK=off go run -C "<loaded-skill-dir>/scripts/resolve_models" . \
  --project-dir <absolute-project-dir>
```

The target is the project being worked on, never the skill installation or an
Easy Loop visit directory. `--project-dir` is required because `GOWORK=off go run -C`
changes directory. Capture the target root before invocation; from its root,
`--project-dir "$PWD"` supplies it explicitly.
`--user-config <absolute-file>` exists for isolated tests; normal runs use the fixed
user path above regardless of CLAUDE_CONFIG_DIR or CODEX_HOME.

The JSON output includes each role's resolved `model`, `cli`, `model_args` argv
list, and per-field `sources`. Report the effective assignments briefly.
Use these exact model/effort arguments in preflight, fresh, resumed, draft,
critique, and execution calls; shell-quote each argument when using a shell.
Do not pass both template defaults and resolved flags. Preserve the skill's
existing permissions, working directories, prompts, and routing.

Easy Loop roles may switch between `claude-` (Claude Code) and `gpt-` (Codex)
models. Choose the CLI and effort flag from the resolved assignment. Resolve
once after startup inputs, save the JSON as `setup/resolved-models.json`, and
reuse it for the whole run including resumed visits. Keep using the original
installed helper when copying `SKILL.md` into the run directory.

The `claude`, `codex`, and `gemini` sprint roles keep their provider lanes.
Their `main` role handles orchestration: planning also uses it for interview
and merge; execution uses it for review. Gemini
uses `agy`; its `cli-default` sentinel preserves agy's configured model by
omitting `--model`. Set `gemini.model` to a concrete `gemini-...` identifier to
pin it. Never pass the sentinel as a model ID.

The `main` role defaults to `inherit`, meaning the invoking agent. It cannot
switch its own model by saying so: when resolved to a concrete model, hand the
skill to one foreground CLI worker using the resolved arguments. Give it the
loaded skill path, original request/arguments, target directory, resolved JSON,
and this marker: "Model-selected main worker: main selection is already
applied; do not delegate main again." That worker performs the skill and its
normal interactions; relay any required user questions and resume the same
worker with the answers. Keep existing scope and permissions. A marked worker
uses the supplied assignments instead of resolving or re-delegating main.

`df-semantic-index` also has `reader`. With `inherit`, readers use the effective
main worker's model; a concrete override applies to every segment reader via
the matching CLI (or a runtime worker that can honor that exact model). Its
normal parallel-read threshold and batch limit still apply. `inherit` accepts
no effort override; choose a concrete model to configure effort.

For `df-promise`, a contract requiring a named model or independent reviewer
still applies. A conflicting override is a configuration conflict to report,
not permission to substitute a different model or discard independence.

This bundled Go port is maintained locally. A borrowed-skill refresh must preserve
its Go entry points and test them; it must not restore executable Python helpers.
