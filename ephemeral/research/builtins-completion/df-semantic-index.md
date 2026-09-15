---
name: df-semantic-index
description: Build, upsert, maintain, rebalance, and evaluate semantic indexes for large local token caches. Use this whenever the user asks to create a semantic index, index a token cache, upsert FROM a source TO an index, reduce retrieval tool calls, route agents through prior art, maintain/rebalance an index tree, run retrieval benchmarks, or record semantic-index housekeeping.
argument-hint: <from> <to> [mode]
---

# Semantic Index

**Two concepts — keep them distinct:**

- **Token cache**: The full local materialization of all tokens relevant to the business: source code, docs, research, schemas, whatever makes up the total business context. Must exist as a real POSIX filesystem so tools like `rg`, `find`, `grep`, and `wc` work on it directly. Can be enormous (hundreds of millions to billions of tokens). Always passed as the `from` argument. If the token cache lives remotely (GitHub, Google Drive, Dropbox, Box, S3), it must be synced locally before this skill runs; the skill reads the token cache but never syncs it.
- **Semantic index**: The compact routing tree this skill builds and maintains over the token cache. Agents use it to sniff and peek at relevant citations without ingesting the full token cache. Passed as the `to` argument. Also lives on the local POSIX filesystem.

Create or maintain a filesystem-based semantic index that helps an agent retrieve the right evidence from a large token cache with fewer tool calls. A semantic index is a routing tree: top-level entrypoints describe the token cache and major routes, interior nodes narrow by task/theme/source family, and leaves contain citations into the token cache.

The skill accepts a token cache path and an index output path:

```text
$ARGUMENTS
```

Interpret the first path-like argument as **from** (the local token cache root) and the second path-like argument as **to** (the index output directory). If the user gives natural language instead of exact paths, resolve likely paths from the repo context and state the assumption before writing. Do not modify the token cache — read it only.

## Model selection

Before model preflight or substantive work, read [references/models.md](references/models.md)
and run the adjacent `scripts/resolve_models.py` for the target project. Apply
skill defaults, then `~/.config/diffusion/skills/models.yaml`, then the project's
`.diffusion/skills/models.yaml`. Use the resolved model and effort for every
role; the reference describes main-worker handoff and prevents recursive handoff.

```yaml
model_defaults:
  main:
    model: inherit
  reader:
    model: inherit
```

## Operating Modes

Choose the mode from the request and index state:

- **scratch build**: `to` does not exist, is empty, or lacks an entrypoint. Build a new index.
- **incremental update**: `to` already has usable routing files. Refresh changed source coverage and preserve working routes.
- **retrieval benchmark**: the user asks for evals, benchmarks, quality, tool-call reduction, or proof that the index retrieves well.
- **rebalance**: the user asks to rebalance, or metrics show routing skew such as overfull leaves, very shallow catch-all files, orphaned source families, or repeated benchmark misses.
- **audit only**: the user asks where things stand or whether an index is healthy. Report metrics and debt without changing content unless asked.

If several modes apply, run them in this order: audit, incremental update or scratch build, benchmark, rebalance. Report each mode as it starts.

## Required Index Contract

The precise storage format is intentionally flexible. Markdown, YAML, JSON, XML, SQLite, and hybrids are all acceptable if the index gives agents a clear retrieval path. Every maintained index should provide these surfaces, regardless of format:

- **Entrypoint**: a root README, INDEX, manifest, or database table that explains scope, token cache root, layout, route order, and known debt.
- **Routing nodes**: intermediate topic/task/source-family nodes that tell the agent which child nodes or adjacent nodes to inspect next.
- **Leaves with citations**: source pointers such as `path:line`, `path:L1-L2`, `file.pdf p.12`, commit/object pointers, URLs, or database ids that resolve back into the token cache.
- **Housekeeping state**: last build/update/benchmark/rebalance timestamps, source path or token cache fingerprint, index metrics, and open debt.
- **Retrieval evals**: at least a small query set or benchmark note once the index is used for serious retrieval.

Prefer human-readable files for small and medium indexes because agents can inspect them directly. Use SQLite or structured machine-readable manifests when the token cache is large enough that queries, metrics, or incremental maintenance would otherwise require too many shell calls.

For format patterns drawn from existing indexes, read `references/index-patterns.md` only when you need examples before choosing a shape.

## Script Paths

The bundled helper scripts live alongside this skill. Resolve `to` first and
set `INDEX_DIR` to that output path so the fallback branch is ready, then locate
the helpers dynamically:

```bash
SKILL_DIR="$(find .agents ~/.agents ~/.codex -maxdepth 5 -type d -name df-semantic-index 2>/dev/null | head -1)"
METRICS_SCRIPT="$SKILL_DIR/scripts/semantic_index_metrics.py"
EVALS_SCRIPT="$SKILL_DIR/scripts/run_evals.py"
if [[ ! -f "$METRICS_SCRIPT" || ! -f "$EVALS_SCRIPT" ]]; then
  : "${INDEX_DIR:?set INDEX_DIR to the resolved semantic-index output path}"
  FALLBACK_DIR="$INDEX_DIR/.semantic-index/tools"
  mkdir -p "$FALLBACK_DIR"
  echo "df-semantic-index helpers not found; created fallback directory, now write the documented inline helpers" >&2
  METRICS_SCRIPT="$FALLBACK_DIR/semantic_index_metrics.py"
  EVALS_SCRIPT="$FALLBACK_DIR/run_evals.py"
fi
```

If the skills directory is not found (e.g., running from a project that doesn't
have `.agents/skills/`), set `INDEX_DIR` to the resolved `to` path and write
minimal inline versions of the needed scripts there before their first use. Preserve
the documented command-line interfaces so every later command remains valid.

## First Pass

1. Parse and confirm `from` and `to`.
2. Inspect the token cache shape cheaply:
   ```bash
   find <from> -maxdepth 2 -type f | sed -n '1,120p'
   find <from> -maxdepth 2 -type d | sed -n '1,120p'
   ```
   Use `rg --files <from>` for larger scans when available.
3. Inspect existing index state if `to` exists:
   ```bash
   find <to> -maxdepth 3 -type f | sort | sed -n '1,160p'
   python3 "$METRICS_SCRIPT" --index <to> --source <from>
   ```
4. State the chosen mode and why in one or two sentences.
5. Count distinct token-cache files or directories. If > 10, plan to use parallel readers (see Parallel Read Harness below) and identify the segment boundaries before writing anything.

## Parallel Read Harness

For token caches with 10 or more distinct token-cache files or directories, use the runtime's available parallel-read mechanism to build leaf nodes in parallel without asking for extra approval. These are low-consequence read-only inspections plus writes to pre-assigned index paths. Each worker reads its assigned segment deeply and writes its leaf files; the orchestrator handles segmentation, routing files, and integration. This pattern cuts wall time dramatically on large token caches and lets each worker fill its context window with a coherent slice rather than spreading attention across unrelated sources.

**When to use**: scratch builds or incremental updates where deep reading of > 20 token-cache files would be needed. Skip for small token caches, rebalancing (surgical), or audit-only runs.

Use whatever the adapter exposes: background workers, parallel agents, batch CLI calls, or read-only worker jobs. If the adapter does not support parallel workers, fall back to sequential segment reads and say so in the report.

### Orchestrator vs Worker Division

| | Orchestrator | Worker |
|---|---|---|
| **Does** | Scan token cache, decide segments, pre-assign output paths, launch batches, integrate leaf files into routing nodes, write entrypoint, run metrics | Read assigned token cache paths deeply, write dense leaf files with citations |
| **Does not** | Read individual token-cache files in depth | Write routing files, touch other segments, create subdirectories beyond the assigned output path |

The orchestrator never reads the full token cache in depth; that is why parallel workers exist.

### Segmentation

After the first pass cheap scan, partition the token cache into semantically coherent segments before launching any workers:

1. Group by **source family**: repos together, papers together, web docs together, discussion threads together. Within a large family, group by closely related topic or subdirectory.
2. Target **3–15 token-cache files per segment** — enough to fill meaningful context, not so many that nothing gets read thoroughly.
3. Ensure **non-overlapping assignments**: each source file belongs to exactly one worker. Cross-segment themes (e.g., "all sources discuss AcroForm") are handled by the orchestrator in routing files after the fact.
4. **Pre-assign output paths** for each worker before launching (e.g., worker 1 writes to `<to>/repos/acroform/`, worker 2 to `<to>/repos/ocr/`). No two workers should write to the same path.
5. Name each segment clearly; the name becomes the worker's framing and eventually a routing node label (e.g., "AcroForm libraries", "OCR + vision pipeline", "web API docs", "prior-art papers").

Apply the resolved `reader` model to every segment worker (see
[model configuration](references/models.md)); `inherit` uses the effective main
model. Sequential fallback readers must honor the same assignment.

### Launching Batches (K ≤ 5)

Launch at most **5 workers in a single batch** so they run concurrently. Wait for all workers in the batch to complete before sending the next batch. Five is the ceiling that balances API rate-limit headroom with meaningful throughput; do not exceed it.

A token cache with 20 segments runs in 4 batches of 5. A token cache with 8 segments runs in 2 batches (5 + 3).

### Worker Prompt Template

Each worker prompt must be fully self-contained; the worker may have no conversation history. Fill in the placeholders before sending:

```
You are building leaf index files for a semantic index.

Segment name: <e.g., "OCR and vision repos">

Corpus paths to read (read these thoroughly):
<list of absolute paths, one per line>

Write output to: <absolute path, e.g., /home/user/project/docs/index/repos/ocr/>
  One .md file per source, or one combined file if sources are small (<5 files total).
  File names should match the source name (e.g., ocrmypdf.md, tesseract.md).

Citation convention:
  Code: path/to/file:line or path/to/file:L10-L25
  PDF:  path/to/file.pdf p.N or pp.N-M
  URL:  verbatim

For each source in your segment:
1. Read it thoroughly (README, key code/doc files, examples).
2. Write a dense leaf file with:
   - Purpose: 1-2 sentence summary of what this source is.
   - Key concepts: bulleted themes, patterns, APIs, techniques — each with a citation anchor.
   - Citations: the most important path:line bookmarks.
   - Themes: cross-cutting patterns this source participates in.
   - Gotchas: pitfalls, version quirks, license restrictions.
   - Recipes: "to do X, start at path:line" — one or two task-oriented hints.
3. Do NOT write routing files (README, topics, themes, taxonomy, index.md).
4. Do NOT read or modify files outside your assigned token-cache paths.
5. Do NOT create subdirectories beyond what is needed for your leaf files.

When done, report:
  - Absolute paths of every file you wrote.
  - One-sentence summary of your segment.
  - Any sources you could not read or found too sparse to index.
```

Adapt the template to the token cache domain — the key invariants are the output path assignment, the citation convention, and the prohibition on writing routing files.

### Integration After All Batches Complete

Once every worker has finished:

1. **Read the leaf tree**: `find <to> -type f | sort` to confirm all expected leaf files exist.
2. **Synthesize routing files**: write README, topics, themes, or taxonomy that reference the leaves and organize them by retrieval intent, not by which worker wrote them. Adjacent-node links belong here (e.g., "AcroForm appearance → see also OCR pipeline").
3. **Verify coverage**: run metrics and confirm `orphan_leaf_count` is near zero (all leaves reachable from routing files).
4. **Write evals.jsonl** with one query per major retrieval intent, pointing at the routing nodes and leaf files the workers wrote.
5. **Write state**:
   ```bash
   python3 "$METRICS_SCRIPT" --index <to> --source <from> --mode scratch-build --write-state
   ```

## Scratch Build

Build for agent retrieval, not source-tree mirroring. A good first shape usually has:

- `README.md`: scope, token cache root, route order, layout, citation conventions, known debt, and housekeeping summary.
- `TAXONOMY.md`, `topics.md`, or `routes.yaml`: the master map from retrieval intent to child nodes.
- `themes/` or `routes/`: cross-cutting concepts that gather multiple source families.
- `sources/`, `repos/`, `papers/`, `web/`, or other leaves: dense per-source annotations with citations.
- `.semantic-index/tools/`: inline fallback helpers when the installed skill bundle cannot be resolved.
- `.semantic-index/state.json`: generated housekeeping metrics and timestamps.
- `.semantic-index/evals.jsonl`: retrieval benchmark queries and expected citation targets (see Eval Format below).

Use a shallow sample before committing to the layout. Read enough files to understand the token cache categories, then create a route tree that answers likely agent questions: "what task am I doing?", "which theme does it touch?", "which source family is authoritative?", and "which exact citations should I open?"

For token caches with 10 or more token-cache files, use the **Parallel Read Harness** pattern above to build leaves, then write routing files after all batches complete. For smaller token caches, build leaves and routing files directly.

After writing all files, run metrics and write state:

```bash
python3 "$METRICS_SCRIPT" --index <to> --source <from> --mode scratch-build --write-state
```

## Incremental Update

Preserve useful routes and update only stale coverage.

1. Compare source and index freshness using git, mtimes, manifests, or current file lists.
2. Identify new, changed, deleted, or under-indexed source areas.
3. If the changed area spans 5 or more distinct token-cache files, use the **Parallel Read Harness** pattern to rebuild those leaves in parallel. Assign only the changed segments; leave untouched segments alone.
4. If the changed area is small (< 5 files), update routing nodes and leaves directly.
5. Keep citations resolvable. If a cited source moved or disappeared, mark it as stale debt or replace it.
6. Refresh housekeeping state:
   ```bash
   python3 "$METRICS_SCRIPT" --index <to> --source <from> --mode incremental-update --write-state
   ```

Do not blindly regenerate everything if the existing routes are useful. Regeneration is appropriate only when the route model is wrong or the token cache changed so much that local patches would be more expensive than a rebuild.

## Retrieval Benchmarks

Benchmarks measure whether an agent can find the right leaves quickly.

### Eval Format

Create or maintain `.semantic-index/evals.jsonl` with one JSON object per line:

```json
{"id": "q001", "query": "how to handle AcroForm appearance streams?", "expected_routes": ["topics.md", "themes.md"], "expected_citations": ["repos/apache-pdfbox.md"], "acceptable_families": ["repos/"], "notes": "optional context"}
```

Fields:
- `id`: short unique identifier (e.g. `"q001"`)
- `query`: the natural-language task a future agent would actually ask
- `expected_routes`: routing files the agent should pass through
- `expected_citations`: leaf file paths the agent should eventually open
- `acceptable_families`: path prefixes where any hit counts (e.g. `"repos/"`)
- `notes`: ambiguity, freshness concerns, false-positive risks

Aim for 10–30 queries covering the main retrieval intents.

### Running Benchmarks

```bash
python3 "$EVALS_SCRIPT" --index <to> --write-results
```

This runs BFS traversal from the entrypoint for each query, scores citation reachability, and writes a timestamped result to `.semantic-index/benchmarks/`.

Reported metrics:
- **success@1**: did any expected citation appear in the traversal?
- **success@3**: did at least 3 expected citations appear?
- **citation_precision**: fraction of expected citations found
- **route_hit_rate**: fraction of queries where routing files were on the path
- **mean_tool_call_estimate**: average files an agent would read per query
- **mean_path_length**: average hops from entrypoint to first relevant hit
- **misses**: query IDs that failed success@1

For manual benchmark runs, report the route walked, citations found, misses, and the next rebalance candidate.

## Rebalancing

Rebalance to improve retrieval metrics, not for aesthetic symmetry. Use a hill-climbing loop:

### Hill-Climbing Loop

1. **Baseline**: run benchmarks and record current aggregate metrics.
   ```bash
   python3 "$EVALS_SCRIPT" --index <to> --write-results
   ```
2. **Identify the worst miss**: pick the eval query with the lowest citation_precision or longest path_length.
3. **Diagnose**: inspect the routing path for that query. Is the relevant leaf orphaned? Is the routing file too dense? Are related concepts split across siblings that should be merged?
4. **Make ONE targeted change**: split a node, merge thin siblings, add an adjacent-node link, introduce a task-first route, or change a routing file's structure.
5. **Re-run benchmarks**: compare new aggregate metrics to baseline.
6. **Keep or revert**: keep the change if success@1, citation_precision, or mean_tool_call_estimate improved. If metrics degraded, revert and try a different change.
7. **Repeat** until metrics plateau (< 2% improvement over 3 iterations) or all high-priority misses are resolved.

### Rebalancing Signals

Look for:
- **Orphan leaves**: leaves not reachable from any routing file (`orphan_leaf_count > 0`).
- **Overfull nodes**: `fan_out.skewed_nodes` listing directories with > 2× mean fan-out — split these.
- **Catch-all routing files**: routing files with `routing_density` (citations/routing-file) >> leaf density — they're mixing too many concepts.
- **Adjacent concepts separated too early**: benchmark misses where the expected citation is close to a routing node that doesn't link to it.
- **High `balance_ratio`**: ratio of max depth to mean leaf depth >> 1.0 means some branches are very deep while others are shallow — flatten or redistribute.
- **Benchmark misses with term_score = 0**: routing files don't mention the query terms at all — add keyword coverage.

Rebalancing actions: split nodes, merge thin siblings, add adjacent-node links, introduce a task-first route above source-family routes, move repeated caveats to a shared node, or change formats.

After rebalancing, re-run metrics and at least the benchmark queries that motivated the change:

```bash
python3 "$METRICS_SCRIPT" --index <to> --source <from> --mode rebalance --write-state
python3 "$EVALS_SCRIPT" --index <to> --write-results
```

## Metrics And Housekeeping

Use the bundled helper for a cheap structural report:

```bash
python3 "$METRICS_SCRIPT" --index <to> --source <from>
```

When maintenance changes are made, write or refresh state:

```bash
python3 "$METRICS_SCRIPT" --index <to> --source <from> --mode <mode> --write-state
```

Key metrics tracked:
- `last_housekeeping_at`, `last_mode`
- token-cache and index paths, file counts by extension
- `max_file_depth`, `mean_leaf_depth`, `balance_ratio` (max/mean, ideal near 1.0)
- `breadth_by_level`: nodes at each tree level
- `fan_out`: mean, max, variance, and skewed (overfull) nodes
- `routing_file_count`, `likely_leaf_count`
- `orphan_leaf_count`: leaves not referenced from any routing file
- `citation_count`, `citation_density_per_leaf`, `routing_density`
- `debt_marker_count`: TODO/FIXME/stale markers
- `benchmark_file_count`, `last_benchmark_at`

Metrics are signals, not policy. A domain-specific index may deliberately be wide, deep, or database-backed. Explain why when the metrics look unusual but the retrieval behavior is good.

## Reporting

While working, report:

- `from`, `to`, and chosen mode.
- What source sample or existing routes you inspected.
- What files or tables you changed.
- Current metrics: depth, breadth, leaf count, citation count, orphan count, fan_out skew, balance_ratio, debt count, and benchmark status.
- Any unresolved source areas or stale citations.
- Suggested next benchmark or rebalance target.

End with a concise status: scratch built, incrementally updated, benchmarked, rebalanced, or audited only.

## Sprint Planning Integration

If this index will be used by the `df-sprint-plan` or `df-sprint-execute` workflow, write or update `docs/SEMANTIC-INDEX.md` in the **project root where sprints are planned** (not inside the index directory). This is the permanent pointer that planning and execution agents read to discover the token cache and semantic index.

The file must capture both the token cache (the full token cache) and the semantic index (the routing tree). Both must be on the local POSIX filesystem — agents need `rg`, `find`, and direct file reads to work.

Use this format:

```markdown
# Semantic Index Configuration

## Token Cache

The token cache is the full local materialization of the project context.
Tools like `rg`, `find`, `grep`, and `wc` operate on it directly.

**Remote source**: <GitHub URL | gdrive:<path> | dropbox:<path> | box:<path> | s3://bucket/prefix | "local only">
**Remote type**: `git` | `rclone-gdrive` | `rclone-dropbox` | `rclone-box` | `s3` | `local`
**Local path**: `/absolute/path/to/token-cache/`
**Sync command**:
```
<exact shell command to materialize or refresh the local token cache>
```
**Token cache scope**: <what this token cache contains and why it's relevant>

## Semantic Index

The semantic index is a routing tree built over the token cache.
Use it to retrieve relevant citations without scanning the full token cache.

**Status**: Available
**Local path**: `/absolute/path/to/index/`
**Entrypoint**: `/absolute/path/to/index/README.md`
**Access**: Read the entrypoint, follow routing nodes, open leaf citations.
Citations in leaf files resolve to paths under the token cache local path.
```

Sync command patterns by remote type:

| Remote type | Sync command |
|---|---|
| `git` | `git clone <url> <local-path>` (first time) / `git -C <local-path> pull` (refresh) |
| `rclone-gdrive` | `rclone sync "gdrive:<path>" "<local-path>"` |
| `rclone-dropbox` | `rclone sync "dropbox:<path>" "<local-path>"` |
| `rclone-box` | `rclone sync "box:<path>" "<local-path>"` |
| `s3` | `aws s3 sync s3://<bucket>/<prefix> <local-path>` |
| `local` | *(already materialized — write `"local only"` for remote source, omit sync command)* |

If the sprint project already has a `docs/SEMANTIC-INDEX.md`, update it rather than creating a duplicate. Tell the user that the file was written or updated so they know planning agents will pick it up on the next sprint run.

## Quality Bar

The index is not done merely because files exist. It is done when a future agent can start at the entrypoint, follow a small number of route decisions, and land on useful citations without scanning the full token cache. Benchmarks confirm this. If the current index would still force filename browsing, keep refining the routes.

Target thresholds for a healthy index:
- `success_at_1 >= 0.8`
- `mean_tool_call_estimate <= 5`
- `orphan_leaf_count / likely_leaf_count < 0.1`
- `balance_ratio < 2.0`
