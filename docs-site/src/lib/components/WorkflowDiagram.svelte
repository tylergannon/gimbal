<script lang="ts">
  interface Props {
    title: string;
    src: string;
  }

  let { title, src }: Props = $props();
  let expanded = $state(false);
</script>

<figure class:expanded aria-label={title}>
  <figcaption class="toolbar">
    <span>{title}</span>
    <button type="button" aria-pressed={expanded} onclick={() => (expanded = !expanded)}>
      {expanded ? "Close" : "Expand"}
    </button>
  </figcaption>
  <div class="viewport">
    <img {src} alt={title} loading="lazy" decoding="async" />
  </div>
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

  img {
    display: block;
    width: auto;
    max-width: 100%;
    height: auto;
    margin-inline: auto;
  }

  figure.expanded {
    position: fixed;
    inset: 1rem;
    z-index: 100;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    margin: 0;
    box-shadow: 0 24px 100px oklch(0 0 0 / 0.75);
  }

  .expanded .viewport {
    max-height: none;
    padding: 2rem;
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
