<script lang="ts">
  import StopIcon from "@lucide/svelte/icons/square";
  import XIcon from "@lucide/svelte/icons/x";
  import * as AlertDialog from "#lib/components/ui/alert-dialog/index.js";
  import type { RunRow, TurnRow } from "../observation/index.svelte.js";

  let {
    open,
    run,
    activeTurn,
    openScopes = 0,
    supervisors = 0,
    busy = false,
    onopenchange,
    onstop,
    oncancel,
  }: {
    open: boolean;
    run: RunRow;
    activeTurn?: TurnRow;
    openScopes?: number;
    supervisors?: number;
    busy?: boolean;
    onopenchange?: (open: boolean) => void;
    onstop?: () => void;
    oncancel?: () => void;
  } = $props();

  const shortID = $derived(run.id.split(".")[0]?.slice(-8) || run.id);
</script>

<AlertDialog.Root {open} onOpenChange={(next) => onopenchange?.(next)}>
  <AlertDialog.Content class="cancel-content">
    <AlertDialog.Header>
      <AlertDialog.Title>Cancel the whole run?</AlertDialog.Title>
      <AlertDialog.Description>
        {run.name} <code>{shortID}</code> stops now. That ends
        {activeTurn ? activeTurn.id : "the active work"}, the planner, and its supervisor sessions.
        The record so far is kept and marked cancelled.
      </AlertDialog.Description>
    </AlertDialog.Header>

    <div class="choices">
      <div class="choice">
        <strong><StopIcon size={14} /> Stop turn</strong>
        <p>
          Ends only {activeTurn?.id ?? "the selected running turn"}. The run continues; the workflow
          decides what to do with the interruption.
        </p>
      </div>
      <div class="choice destructive">
        <strong><XIcon size={14} /> Cancel run</strong>
        <p>
          Ends every scope, session, and command in this run: {openScopes} scopes open,
          {activeTurn ? " 1 turn running" : " no turn running"}, {supervisors} supervisors watching.
        </p>
      </div>
    </div>

    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={busy}>Keep running</AlertDialog.Cancel>
      <AlertDialog.Action
        variant="outline"
        disabled={busy || !activeTurn}
        onclick={() => onstop?.()}
      >
        <StopIcon data-icon="inline-start" size={16} />Stop turn instead
      </AlertDialog.Action>
      <AlertDialog.Action variant="destructive" disabled={busy} onclick={() => oncancel?.()}>
        {busy ? "Cancelling…" : "Cancel run"}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<style>
  :global(.cancel-content) {
    max-width: 540px;
    gap: 16px;
  }

  code {
    font-family: var(--font-mono);
  }

  .choices {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .choice {
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: calc(var(--radius) - 2px);
  }

  .choice.destructive {
    border-color: color-mix(in oklch, var(--destructive) 45%, transparent);
  }

  .choice strong {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    line-height: 18px;
  }

  .choice.destructive strong {
    color: var(--destructive);
  }

  .choice p {
    margin: 4px 0 0;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
  }

  @media (max-width: 620px) {
    .choices {
      grid-template-columns: 1fr;
    }
  }
</style>
