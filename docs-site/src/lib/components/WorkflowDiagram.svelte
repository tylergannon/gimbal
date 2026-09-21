<script lang="ts">
  interface Prompt {
    role: string;
    text: string;
  }

  interface Props {
    title: string;
    src: string;
    prompts?: Prompt[];
  }

  let { title, src, prompts = [] }: Props = $props();
  let expanded = $state(false);
  let viewport: HTMLDivElement;

  function centerDiagram() {
    requestAnimationFrame(() => {
      viewport.scrollLeft = Math.max(0, (viewport.scrollWidth - viewport.clientWidth) / 2);
    });
  }

  function toggleExpanded() {
    expanded = !expanded;
    centerDiagram();
  }
</script>

<figure class:expanded aria-label={title}>
  <figcaption class="toolbar">
    <span>{title}</span>
    <button type="button" aria-pressed={expanded} onclick={toggleExpanded}>
      {expanded ? "Close" : "Expand"}
    </button>
  </figcaption>
  <div class="viewport" bind:this={viewport}>
    <img {src} alt={title} loading="lazy" decoding="async" onload={centerDiagram} />
  </div>
  {#if prompts.length > 0}
    <details class="prompts">
      <summary>Full prompts ({prompts.length})</summary>
      <div class="prompt-list">
        {#each prompts as prompt, index (`${prompt.role}-${index}`)}
          <article>
            <h3>{prompt.role}</h3>
            <p>{prompt.text}</p>
          </article>
        {/each}
      </div>
    </details>
  {/if}
</figure>

<style>
  figure {
    overflow: hidden;
    border: 1px solid var(--border);
    border-radius: 0.9rem;
    background: oklch(0.105 0.008 55);
    box-shadow: inset 0 1px oklch(1 0 0 / 0.04);
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    min-height: 2.75rem;
    padding: 0.6rem 0.85rem;
    border-bottom: 1px solid var(--border);
    background: oklch(0.175 0.014 55);
  }

  figcaption span {
    margin: 0;
    color: var(--muted-foreground);
    font-family: var(--font-mono);
    font-size: 0.72rem;
    letter-spacing: 0.04em;
  }

  button {
    flex: none;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.3rem 0.7rem;
    color: var(--foreground);
    background: var(--secondary);
    font: 600 0.7rem/1 var(--font-sans);
    cursor: pointer;
  }

  button:hover {
    border-color: var(--primary);
    color: var(--primary);
  }

  .viewport {
    max-height: 48rem;
    overflow: auto;
    padding: 1rem;
  }

  .prompts {
    border-top: 1px solid var(--border);
    background: oklch(0.135 0.01 55);
  }

  .prompts summary {
    padding: 0.75rem 0.9rem;
    color: var(--muted-foreground);
    font: 600 0.75rem/1.3 var(--font-mono);
    cursor: pointer;
  }

  .prompt-list {
    display: grid;
    gap: 0.75rem;
    max-height: 28rem;
    overflow: auto;
    padding: 0 0.9rem 0.9rem;
  }

  article {
    padding: 0.85rem;
    border: 1px solid var(--border);
    border-radius: 0.65rem;
    background: oklch(0.105 0.008 55);
  }

  article h3 {
    margin: 0 0 0.45rem;
    color: var(--primary);
    font: 600 0.75rem/1.3 var(--font-mono);
  }

  article p {
    margin: 0;
    color: var(--foreground);
    font: 400 0.84rem/1.55 var(--font-sans);
    white-space: pre-wrap;
  }

  img {
    display: block;
    width: auto;
    max-width: none;
    height: auto;
    margin-inline: auto;
  }

  figure.expanded {
    position: fixed;
    inset: 1rem;
    z-index: 100;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    margin: 0;
    box-shadow: 0 24px 100px oklch(0 0 0 / 0.75);
  }

  .expanded .viewport {
    max-height: none;
    padding: 2rem;
  }

  .expanded .prompt-list {
    max-height: 35vh;
  }

  @media (max-width: 40rem) {
    .viewport {
      max-height: 30rem;
      padding: 0.75rem;
    }

    figure.expanded {
      inset: 0.5rem;
    }

    .expanded .viewport {
      padding: 1rem;
    }
  }
</style>
