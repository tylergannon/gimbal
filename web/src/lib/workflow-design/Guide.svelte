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
 article{max-width:70ch;color:#0b0c0c}h2{font-size:22px;font-weight:650;margin:0 0 20px}h3{font-size:17px;font-weight:650;margin:28px 0 10px}p,dd{font-size:15px;line-height:1.65;white-space:pre-line;margin:0 0 16px}pre{padding:12px;background:#f3f5f7;border:1px solid #a6acb0;border-radius:4px;font:13px/1.6 ui-monospace,monospace;white-space:pre-wrap;overflow-wrap:anywhere}code{font-family:ui-monospace,monospace;font-size:.9em}dt{font-weight:600;margin:16px 0 4px}dt span{font-size:12px;font-weight:400;color:#484949}dd{margin-left:0}details{margin-top:24px}summary{cursor:pointer;font-size:14px}.source p{font-size:13px;margin:8px 0}summary:focus-visible{outline:3px solid #0b0c0c;outline-offset:2px}
</style>
