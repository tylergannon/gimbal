<script module lang="ts">
  import type { ScopeRow } from "../observation/index.js";
  import type { NodeOperation } from "./Node.svelte";
  import type { PipState } from "./Pip.svelte";

  export type SheetKind = "scope" | "group" | "loop";
  export type FoldedStep = { operation: NodeOperation; state: PipState };
  export type SheetSelection = (scope: ScopeRow) => void;

  export function countWrittenContextKeys(scope: ScopeRow) {
    const keys = new Set(Object.keys(scope.values));
    if (scope.task !== undefined) keys.add("task");
    return keys.size;
  }

  export function resolveScopeInstance(
    scope: ScopeRow,
    instances: ScopeRow[],
    selectedInstance?: string,
  ) {
    const latest = instances.reduce(
      (current, instance) => (instance.began > current.began ? instance : current),
      instances.at(-1) ?? scope,
    );
    return instances.find((item) => item.key === selectedInstance) ?? latest;
  }
</script>

<script lang="ts">
  import BoxIcon from "@lucide/svelte/icons/box";
  import BracesIcon from "@lucide/svelte/icons/braces";
  import BotIcon from "@lucide/svelte/icons/bot";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import GitForkIcon from "@lucide/svelte/icons/git-fork";
  import InterviewIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import RepeatIcon from "@lucide/svelte/icons/repeat-2";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import type { Snippet } from "svelte";
  import * as Select from "#lib/components/ui/select/index.js";
  import Pip from "./Pip.svelte";

  let {
    scope,
    instances = [scope],
    selectedInstance,
    kind = "scope",
    depth = 0,
    selected = false,
    selectionPath = false,
    folded = false,
    root = false,
    foldable = true,
    contextTotal = 0,
    elapsed = "",
    steps = [],
    onselect,
    oninstancechange,
    onopen,
    onfold,
    children,
  }: {
    scope: ScopeRow;
    instances?: ScopeRow[];
    selectedInstance?: string;
    kind?: SheetKind;
    depth?: number;
    selected?: boolean;
    selectionPath?: boolean;
    folded?: boolean;
    root?: boolean;
    foldable?: boolean;
    contextTotal?: number;
    elapsed?: string;
    steps?: FoldedStep[];
    onselect?: SheetSelection;
    oninstancechange?: SheetSelection;
    onopen?: SheetSelection;
    onfold?: SheetSelection;
    children?: Snippet;
  } = $props();

  let localSelectedInstance = $state<string | undefined>(undefined);

  const activeScope = $derived(
    resolveScopeInstance(scope, instances, selectedInstance ?? localSelectedInstance),
  );
  const instanceIndex = $derived(instances.findIndex((item) => item.key === activeScope.key));
  const repeated = $derived(instances.length > 1);
  const isFolded = $derived(root ? false : folded);
  const scopePipState = $derived(scopeState(activeScope));
  const contextWritten = $derived(countWrittenContextKeys(activeScope));

  function scopeState(row: ScopeRow): PipState {
    if (row.error) return "failed";
    return row.status === "running" ? "running" : "ended";
  }

  function instanceLabel(index: number) {
    return `${activeScope.name} ${index + 1} of ${instances.length}`;
  }

  function selectInstance(key: string) {
    const next = instances.find((item) => item.key === key);
    if (next) {
      localSelectedInstance = key;
      oninstancechange?.(next);
    }
  }

  function selectSheet() {
    onselect?.(activeScope);
    if (isFolded) onopen?.(activeScope);
  }
</script>

<section
  class="sheet"
  class:root
  class:selected
  class:path={selectionPath}
  class:folded={isFolded}
  data-depth={depth % 2}
  data-scope={activeScope.key}
>
  {#if !root}
    <div class="heading">
      {#if repeated}
        <Select.Root type="single" value={activeScope.key} onValueChange={selectInstance}>
          <Select.Trigger
            class="sheet-trigger"
            aria-label={`Select ${activeScope.name} instance`}
          >
            <Pip state={scopePipState} />
            <span>{instanceLabel(instanceIndex)}</span>
          </Select.Trigger>
          <Select.Content>
            {#each instances as instance, index}
              <Select.Item value={instance.key} label={instanceLabel(index)}>
                <Pip state={scopeState(instance)} />
                {instanceLabel(index)}
              </Select.Item>
            {/each}
          </Select.Content>
        </Select.Root>
      {:else}
        <button type="button" class="label" onclick={selectSheet}>
          {#if kind === "group"}
            <GitForkIcon size={13} />
          {:else if kind === "loop"}
            <RepeatIcon size={13} />
          {:else}
            <BoxIcon size={13} />
          {/if}
          <span>{activeScope.name}</span>
        </button>
      {/if}

      {#if contextTotal > 0 && !isFolded}
        <button
          type="button"
          class="context"
          aria-label={`${contextWritten} of ${contextTotal} context keys written`}
          onclick={() => onselect?.(activeScope)}
        >
          <BracesIcon size={12} />
          <span>context</span>
          <span class="secondary">{contextWritten} of {contextTotal}</span>
        </button>
      {/if}

      {#if foldable}
        <button
          type="button"
          class="fold-toggle"
          aria-label={isFolded ? `Open ${activeScope.name}` : `Fold ${activeScope.name}`}
          onclick={() => (isFolded ? onopen?.(activeScope) : onfold?.(activeScope))}
        >
          {#if isFolded}
            <ChevronRightIcon size={14} />
          {:else}
            <ChevronDownIcon size={14} />
          {/if}
        </button>
      {/if}
    </div>
  {/if}

  {#if isFolded}
    <button type="button" class="fold-summary" onclick={selectSheet}>
      <span class="glyphs">
        {#each steps as step}
          <span class="glyph" title={step.operation.kind.replace("_", " ")}>
            {#if step.operation.kind === "agent_call"}
              <BotIcon size={13} />
            {:else if step.operation.kind === "command"}
              <TerminalIcon size={13} />
            {:else}
              <InterviewIcon size={13} />
            {/if}
            <Pip state={step.state} />
          </span>
        {/each}
      </span>
      <span class="summary-status">
        <Pip state={scopePipState} />{scopePipState.replace("-", " ")}
      </span>
      {#if elapsed}<span class="elapsed">{elapsed}</span>{/if}
    </button>
  {:else}
    <div class="content">
      {@render children?.()}
    </div>
  {/if}
</section>

<style>
  .sheet {
    position: relative;
    min-width: 0;
    box-sizing: border-box;
    color: var(--foreground);
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 10px;
  }

  .sheet[data-depth="1"] {
    background: var(--map-paper-2);
  }

  .sheet.path {
    border-color: var(--map-line);
    box-shadow: var(--map-shadow-path);
  }

  .sheet.selected {
    z-index: 1;
    background: var(--map-paper-2);
    border: 1.5px solid var(--map-line-strong);
    box-shadow: var(--map-shadow-selected);
  }

  .sheet.root {
    background: transparent;
    border: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .heading {
    position: absolute;
    z-index: 2;
    top: -13px;
    right: 10px;
    left: 10px;
    display: flex;
    height: 26px;
    align-items: center;
    gap: 8px;
    pointer-events: none;
  }

  .label,
  .context,
  .fold-toggle {
    pointer-events: auto;
  }

  .label {
    display: inline-flex;
    height: 24px;
    box-sizing: border-box;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
    color: var(--foreground);
    font: inherit;
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    white-space: nowrap;
    cursor: pointer;
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 7px;
  }

  .sheet[data-depth="1"] .label,
  .sheet.selected .label {
    background: var(--map-paper-2);
  }

  .sheet.path .label {
    border-color: var(--map-line);
  }

  .sheet.selected .label {
    border: 1.5px solid var(--map-line-strong);
  }

  .sheet :global([data-slot="select-trigger"].sheet-trigger) {
    height: 24px;
    gap: 7px;
    padding: 0 7px 0 10px;
    color: var(--foreground);
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    background: var(--map-paper-1);
    border: 1px solid var(--map-line-soft);
    border-radius: 7px;
    box-shadow: var(--shadow-xs);
    pointer-events: auto;
  }

  .sheet[data-depth="1"] :global([data-slot="select-trigger"].sheet-trigger),
  .sheet.selected :global([data-slot="select-trigger"].sheet-trigger) {
    background: var(--map-paper-2);
  }

  .sheet.path :global([data-slot="select-trigger"].sheet-trigger) {
    border-color: var(--map-line);
  }

  .sheet.selected :global([data-slot="select-trigger"].sheet-trigger) {
    border: 1.5px solid var(--map-line-strong);
  }

  .label:focus-visible,
  .context:focus-visible,
  .fold-toggle:focus-visible,
  .fold-summary:focus-visible {
    outline: 2px solid var(--status-live);
    outline-offset: 2px;
  }

  .context {
    display: inline-flex;
    height: 24px;
    box-sizing: border-box;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    padding: 0 8px;
    color: var(--foreground);
    font: inherit;
    font-size: 13px;
    font-weight: 500;
    line-height: 18px;
    white-space: nowrap;
    cursor: pointer;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line);
    border-radius: 7px;
  }

  .secondary,
  .elapsed {
    color: var(--status-muted);
  }

  .fold-toggle {
    display: inline-flex;
    width: 24px;
    height: 24px;
    align-items: center;
    justify-content: center;
    padding: 0;
    color: var(--status-muted);
    cursor: pointer;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line-soft);
    border-radius: 7px;
  }

  .content {
    display: flex;
    min-height: 56px;
    flex-direction: column;
    gap: 12px;
    padding: 32px 8px 8px;
  }

  .root > .content {
    min-height: 0;
    padding: 0;
  }

  .fold-summary {
    display: flex;
    width: 100%;
    min-height: 64px;
    box-sizing: border-box;
    align-items: center;
    gap: 12px;
    padding: 22px 12px 10px;
    color: var(--foreground);
    font: inherit;
    font-size: 13px;
    line-height: 18px;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
  }

  .glyphs {
    display: inline-flex;
    min-width: 0;
    flex: 1;
    align-items: center;
    gap: 4px;
  }

  .glyph {
    display: inline-flex;
    height: 26px;
    align-items: center;
    gap: 4px;
    padding: 0 5px;
    color: var(--status-muted);
    background: var(--map-paper-2);
    border: 1px solid var(--map-line-soft);
    border-radius: 6px;
  }

  .glyph :global(.pip) {
    width: 8px;
    height: 8px;
  }

  .summary-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-weight: 600;
  }

  .elapsed {
    font-family: var(--font-mono);
    white-space: nowrap;
  }
</style>
