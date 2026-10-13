<script lang="ts">
 import type {SourcePage} from './types';
 let {page}:{page:SourcePage}=$props();
 let paragraphs=$derived((page.guide?.prose ?? '').split(/\n\s*\n/));
</script>
<article>
 <h2>Using {page.name}</h2>
 {#each paragraphs as paragraph}{#if paragraph.trim()}<p>{paragraph}</p>{/if}{/each}
 <h3>Invocation</h3><pre>{page.guide?.invocation}</pre>
 <p>Use <code>gimbal run {page.name} --help</code> for instance, project and model options.</p>
 {#if page.guide?.parameters.length}<h3>Inputs</h3><dl>{#each page.guide.parameters as parameter}<dt><code>--{parameter.flag}</code> <span>{parameter.required?'required':'optional'} · {parameter.kind}</span></dt><dd>{parameter.description}</dd>{/each}</dl>{/if}
 {#if page.guide?.roles.length}<details><summary>Roles and default models</summary><dl>{#each page.guide.roles as role}<dt>{role.name}</dt><dd><code>{role.default}</code></dd>{/each}</dl></details>{/if}
 <details class="source"><summary>Source version</summary><p>{page.source.file}:{page.source.line}</p><p>Snapshot <code>{page.guide?.snapshot.slice(0,12)}</code></p></details>
</article>
<style>
 article{max-width:70ch;color:var(--foreground)}h2{font-size:var(--text-xl);font-weight:var(--font-weight-semibold);margin:0 0 20px}h3{font-size:var(--text-base);font-weight:var(--font-weight-semibold);margin:28px 0 10px}p,dd{font-size:15px;line-height:1.65;white-space:pre-line;margin:0 0 16px}pre{padding:12px;background:var(--code);border:1px solid var(--border);border-radius:var(--radius-lg);font:13px/1.6 var(--font-mono);white-space:pre-wrap;overflow-wrap:anywhere}code{font-family:var(--font-mono);font-size:.9em}dt{font-weight:600;margin:16px 0 4px}dt span{font-size:12px;font-weight:400;color:var(--muted-foreground)}dd{margin-left:0}details{margin-top:24px}summary{cursor:pointer;font-size:14px}.source p{font-size:13px;margin:8px 0}summary:focus-visible{outline:2px solid var(--ring);outline-offset:2px}
</style>
