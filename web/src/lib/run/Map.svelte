<script module lang="ts">
  import type {
    CommandRow,
    InterviewRow,
    ScopeRow,
    TurnRow,
  } from "../observation/index.js";
  import type { Supervisor } from "../workflow/types.js";
  import type { NodeOperation } from "./Node.svelte";

  export type MapSelection =
    | { kind: "sheet"; scope: ScopeRow }
    | { kind: "instance"; scope: ScopeRow }
    | {
        kind: "node";
        scope: ScopeRow;
        operation: NodeOperation;
        runtime?: TurnRow | CommandRow | InterviewRow;
      }
    | {
        kind: "watcher";
        scope: ScopeRow;
        supervisor: Supervisor;
        turn?: TurnRow;
      };
</script>

<script lang="ts">
  import type { Graph } from "../workflow/types.js";
  import type { RunSnapshot } from "../observation/index.js";
  import Group from "./Group.svelte";
  import Loop from "./Loop.svelte";
  import Node from "./Node.svelte";
  import Sheet from "./Sheet.svelte";
  import Watcher from "./Watcher.svelte";
  import { buildMapLayout } from "./layout.js";

  let {
    graph,
    snapshot,
    onselect,
    oninstancechange,
  }: {
    graph: Graph;
    snapshot: RunSnapshot;
    onselect?: (selection: MapSelection) => void;
    oninstancechange?: (scope: ScopeRow) => void;
  } = $props();

  let selectedInstances = $state<Record<string, string>>({});
  let foldedScopes = $state<string[]>([]);
  let selectedKey = $state<string | undefined>(undefined);

  const layout = $derived(
    buildMapLayout(graph, snapshot, { selectedInstances, foldedScopes, selectedKey }),
  );

  function selectSheet(sheet: (typeof layout.sheets)[number]) {
    selectedKey = sheet.selectionKey;
    onselect?.({ kind: "sheet", scope: sheet.scope });
  }

  function changeInstance(sheet: (typeof layout.sheets)[number], scope: ScopeRow) {
    selectedInstances = { ...selectedInstances, [sheet.instanceGroupKey]: scope.key };
    if (foldedScopes.includes(sheet.scope.key)) {
      foldedScopes = [...foldedScopes.filter((key) => key !== sheet.scope.key), scope.key];
    }
    selectedKey = `sheet:${scope.key}`;
    oninstancechange?.(scope);
    onselect?.({ kind: "instance", scope });
  }

  function openSheet(sheet: (typeof layout.sheets)[number]) {
    const siblingKeys = layout.sheets
      .filter(
        (candidate) =>
          candidate.parentScopeKey === sheet.parentScopeKey &&
          candidate.scope.key !== sheet.scope.key,
      )
      .map((candidate) => candidate.scope.key);
    foldedScopes = Array.from(
      new Set([
        ...foldedScopes.filter((key) => key !== sheet.scope.key),
        ...siblingKeys,
      ]),
    );
    selectSheet(sheet);
  }

  function foldSheet(sheet: (typeof layout.sheets)[number]) {
    if (!foldedScopes.includes(sheet.scope.key)) {
      foldedScopes = [...foldedScopes, sheet.scope.key];
    }
    selectSheet(sheet);
  }

  function selectNode(node: (typeof layout.nodes)[number]) {
    const scope = snapshot.scopes[node.scopeKey];
    if (!scope) return;
    selectedKey = node.selectionKey;
    onselect?.({
      kind: "node",
      scope,
      operation: node.operation,
      runtime: node.runtime,
    });
  }

  function selectWatcher(watcher: (typeof layout.watchers)[number]) {
    const scope = snapshot.scopes[watcher.watchedScope];
    if (!scope) return;
    selectedKey = watcher.selectionKey;
    onselect?.({
      kind: "watcher",
      scope,
      supervisor: watcher.supervisor,
      turn: watcher.turn,
    });
  }
</script>

<div class="map" role="region" aria-label={`${graph.name} workflow map`}>
  <div class="canvas" style:width={`${layout.width}px`} style:height={`${layout.height}px`}>
    <svg
      class="connections"
      width={layout.width}
      height={layout.height}
      viewBox={`0 0 ${layout.width} ${layout.height}`}
      aria-hidden="true"
    >
      {#each layout.connections as path}
        <path class="spine" d={path} />
      {/each}
      {#each layout.watcherBrackets as path}
        <path class="watcher-bracket" d={path} />
      {/each}
      <circle class="endpoint" cx={layout.start.x} cy={layout.start.y} r="4" />
      <circle class="endpoint" cx={layout.end.x} cy={layout.end.y} r="4" />
    </svg>

    {#each layout.sheets as sheet}
      <div
        class="placed sheet"
        style:left={`${sheet.x}px`}
        style:top={`${sheet.y}px`}
        style:width={`${sheet.width}px`}
        style:height={`${sheet.height}px`}
        style:z-index={sheet.depth}
      >
        <Sheet
          scope={sheet.scope}
          instances={sheet.instances}
          kind={sheet.kind}
          depth={sheet.depth}
          selected={sheet.selected}
          selectionPath={sheet.selectionPath}
          contextTotal={sheet.contextTotal}
          selectedInstance={sheet.scope.key}
          folded={sheet.folded}
          elapsed={sheet.elapsed}
          steps={sheet.steps}
          onselect={() => selectSheet(sheet)}
          oninstancechange={(scope) => changeInstance(sheet, scope)}
          onopen={() => openSheet(sheet)}
          onfold={() => foldSheet(sheet)}
        >
          <div class="sheet-space" style:height={`${Math.max(24, sheet.height - 40)}px`}></div>
        </Sheet>
      </div>
    {/each}

    {#each layout.groups as group}
      <div class="placed structural" style:left={`${group.x}px`} style:top={`${group.y}px`}>
        <Group {...group} />
      </div>
    {/each}

    {#each layout.loops as loop}
      <div class="placed structural" style:left={`${loop.x}px`} style:top={`${loop.y}px`}>
        <Loop {...loop} />
      </div>
    {/each}

    {#each layout.nodes as node}
      <div
        class="placed"
        style:left={`${node.x}px`}
        style:top={`${node.y}px`}
        style:width={`${node.width}px`}
        style:height={`${node.height}px`}
      >
        <Node
          operation={node.operation}
          state={node.state}
          meta={node.meta}
          selected={node.selected}
          small={node.operation.kind === "command"}
          onselect={() => selectNode(node)}
        />
      </div>
    {/each}

    {#if layout.watchers.length > 0}
      <span class="watching" style:top={`${layout.watchers[0].y - 24}px`}>watching</span>
    {/if}
    {#each layout.watchers as watcher}
      <div
        class="placed"
        style:left={`${watcher.x}px`}
        style:top={`${watcher.y}px`}
        style:width={`${watcher.width}px`}
        style:height={`${watcher.height}px`}
      >
        <Watcher
          supervisor={watcher.supervisor}
          state={watcher.state}
          selected={watcher.selected}
          onselect={() => selectWatcher(watcher)}
        />
      </div>
    {/each}

    <span class="end" style:left={`${layout.end.x + 13}px`} style:top={`${layout.end.y - 9}px`}>
      end
    </span>
  </div>
</div>

<style>
  .map {
    position: relative;
    width: 100%;
    min-height: 360px;
    overflow: auto;
    color: var(--foreground);
    background-color: color-mix(in oklch, var(--background) 96%, var(--foreground));
    background-image: radial-gradient(color-mix(in oklch, var(--foreground) 20%, transparent) 1px, transparent 1px);
    background-size: 20px 20px;
  }

  .canvas {
    position: relative;
    margin: 0 auto;
  }

  .placed {
    position: absolute;
  }

  .structural {
    pointer-events: none;
  }

  .sheet {
    z-index: 0;
  }

  .sheet > :global(.sheet) {
    height: 100%;
  }

  .sheet-space {
    width: 1px;
  }

  .connections {
    position: absolute;
    z-index: 2;
    inset: 0;
    overflow: visible;
    pointer-events: none;
  }

  .spine {
    fill: none;
    stroke: var(--status-muted);
    stroke-width: 2;
  }

  .watcher-bracket {
    fill: none;
    stroke: var(--status-muted);
    stroke-width: 1.5;
    stroke-dasharray: 2 3;
  }

  .endpoint {
    fill: var(--status-muted);
  }

  .placed:not(.sheet) {
    z-index: 3;
  }

  .watching,
  .end {
    position: absolute;
    z-index: 4;
    color: var(--status-muted);
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    white-space: nowrap;
  }

  .watching {
    left: 16px;
  }
</style>
