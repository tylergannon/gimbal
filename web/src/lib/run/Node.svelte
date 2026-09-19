<script module lang="ts">
  import type { AgentCall, Command, Interview } from "../workflow/types.js";

  export type NodeOperation =
    | (AgentCall & { kind: "agent_call" })
    | (Command & { kind: "command" })
    | (Interview & { kind: "interview" });
</script>

<script lang="ts">
  import BotIcon from "@lucide/svelte/icons/bot";
  import InterviewIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import Pip, { type PipState } from "./Pip.svelte";

  let {
    operation,
    state,
    meta = "",
    selected = false,
    small = false,
    onselect,
  }: {
    operation: NodeOperation;
    state: PipState;
    meta?: string;
    selected?: boolean;
    small?: boolean;
    onselect?: (operation: NodeOperation) => void;
  } = $props();

  const name = $derived(operation.kind === "agent_call" ? operation.session : operation.name);
</script>

<button
  type="button"
  class="node"
  class:selected
  class:small
  aria-pressed={selected}
  aria-label={`Select ${name}`}
  onclick={() => onselect?.(operation)}
>
  <span class="icon" aria-hidden="true">
    {#if operation.kind === "agent_call"}
      <BotIcon size={14} />
    {:else if operation.kind === "command"}
      <TerminalIcon size={14} />
    {:else}
      <InterviewIcon size={14} />
    {/if}
  </span>
  <span class="name">{name}</span>
  {#if meta}
    <span class="meta">{meta}</span>
  {/if}
  <span class="spacer"></span>
  <Pip {state} />
</button>

<style>
  .node {
    display: flex;
    width: 100%;
    height: 44px;
    box-sizing: border-box;
    align-items: center;
    gap: 10px;
    padding: 0 12px 0 8px;
    color: var(--card-foreground);
    font: inherit;
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
    background: var(--map-paper-2);
    border: 1px solid var(--map-line);
    border-radius: 8px;
    box-shadow: var(--shadow-xs);
  }

  .node:hover {
    border-color: var(--map-line-strong);
  }

  .node:focus-visible {
    outline: 2px solid var(--status-live);
    outline-offset: 2px;
  }

  .node.selected {
    border-color: var(--status-running);
    box-shadow: 0 0 0 3px var(--map-live-soft), var(--shadow-xs);
  }

  .icon {
    display: inline-flex;
    width: 26px;
    height: 26px;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    color: var(--foreground);
    background: var(--muted);
    border-radius: calc(var(--radius) - 4px);
  }

  .name {
    overflow: hidden;
    font-size: 15px;
    font-weight: 600;
    line-height: 20px;
    letter-spacing: -0.005em;
    text-overflow: ellipsis;
  }

  .meta {
    overflow: hidden;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
    text-overflow: ellipsis;
  }

  .spacer {
    flex-grow: 1;
  }

  .node.small {
    height: 36px;
    gap: 8px;
    padding: 0 10px 0 8px;
  }

  .node.small .icon {
    width: 20px;
    height: 20px;
    color: var(--status-muted);
    background: transparent;
  }

  .node.small .name {
    font-family: var(--font-mono);
    font-size: 15px;
    font-weight: 500;
    line-height: 18px;
  }
</style>
