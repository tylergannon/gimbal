<script module lang="ts">
  export type SmallState = "empty" | "no-graph";
</script>

<script lang="ts">
  import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
  import SearchXIcon from "@lucide/svelte/icons/search-x";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import * as Card from "#lib/components/ui/card/index.js";

  let {
    state,
    workflowName = "workflow",
    graphProblem = "missing",
  }: {
    state: SmallState;
    workflowName?: string;
    graphProblem?: "missing" | "mismatch";
  } = $props();
</script>

<Card.Root class="state-card">
  {#if state === "empty"}
    <Card.Header>
      <Card.Title>Empty project</Card.Title>
      <Card.Description>Runs are started from the terminal, so this state points there.</Card.Description>
    </Card.Header>
    <Card.Content>
      <div class="empty-state">
        <span class="state-icon"><TerminalIcon size={18} /></span>
        <strong>No runs yet</strong>
        <span>Start a workflow from the terminal. It appears here while it runs.</span>
      </div>
    </Card.Content>
  {:else}
    <Card.Header>
      <Card.Title>Workflow graph required</Card.Title>
      <Card.Description>This run cannot be mapped by the workflow graph compiled into this server.</Card.Description>
    </Card.Header>
    <Card.Content class="no-graph-state">
      <div class="message" role="status">
        <CircleAlertIcon size={16} />
        <div>
          <strong>
            {graphProblem === "mismatch"
              ? "The registered graph does not match this run"
              : `No generated graph is registered for ${workflowName}`}
          </strong>
          <span>
            Regenerate the workflow, then rebuild and restart the binary serving this project.
          </span>
        </div>
      </div>
      <div class="missing-run">
        <SearchXIcon size={16} />
        <span>
          Run <code>go generate ./...</code> from the module that owns <code>{workflowName}</code>.
          Its generated file must call <code>gimble.RegisterGraph</code> for this workflow.
        </span>
      </div>
      <div class="missing-run">
        <TerminalIcon size={16} />
        <span>
          Rebuild the serving binary and restart it. In the Gimble checkout, use
          <code>just build</code>.
        </span>
      </div>
      <a class="back-link" href="/">Back to runs</a>
    </Card.Content>
  {/if}
</Card.Root>

<style>
  :global(.state-card) {
    box-sizing: border-box;
    width: 100%;
    min-height: 250px;
    color: var(--foreground);
    background: var(--card);
    border: 1px solid var(--map-line);
  }

  :global(.state-card [data-slot="card-title"]) {
    font-size: 15px;
  }

  .empty-state {
    display: flex;
    min-height: 130px;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    gap: 7px;
    padding: 14px;
    color: var(--status-muted);
    font-size: 13px;
    line-height: 18px;
    text-align: center;
    border: 1px dashed var(--map-line);
    border-radius: var(--radius);
  }

  .empty-state strong {
    color: var(--foreground);
    font-size: 14px;
  }

  .state-icon {
    display: inline-flex;
    width: 34px;
    height: 34px;
    align-items: center;
    justify-content: center;
    color: var(--foreground);
    background: var(--muted);
    border-radius: calc(var(--radius) - 2px);
  }

  :global(.no-graph-state) {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .missing-run {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .message {
    display: grid;
    grid-template-columns: 16px 1fr;
    gap: 9px;
    padding: 11px 12px;
    font-size: 13px;
    line-height: 18px;
    background: color-mix(in oklch, var(--card) 70%, var(--muted));
    border: 1px solid var(--map-line);
    border-radius: calc(var(--radius) - 2px);
  }

  .message div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .message span,
  .missing-run {
    color: var(--status-muted);
  }

  .back-link {
    align-self: flex-start;
  }

  .back-link {
    display: inline-flex;
    height: 32px;
    align-items: center;
    justify-content: center;
    padding: 0 12px;
    color: var(--foreground);
    font-size: 13px;
    font-weight: 500;
    text-decoration: none;
    border: 1px solid var(--border);
    border-radius: 6px;
  }

  .missing-run {
    align-items: flex-start;
    font-size: 13px;
    line-height: 20px;
  }

  .missing-run :global(svg) {
    flex-shrink: 0;
    margin-top: 2px;
  }

  code {
    color: var(--foreground);
    font-family: var(--font-mono);
  }
</style>
