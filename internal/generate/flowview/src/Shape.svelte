<script lang="ts">
  import Shape from './Shape.svelte';
  import type { ValueShape } from './types';
  let {name,shape,path,openPaths=[],ondetail,optional=false,goTypes=false}: {
    name:string;shape:ValueShape;path:string;openPaths?:string[];ondetail:(path:string,open:boolean)=>void;optional?:boolean;goTypes?:boolean
  }=$props();
  const escapePath=(name:string)=>name.replaceAll('~','~0').replaceAll('/','~1');
  let resolved=$derived(shape.kind==='pointer' && shape.element ? shape.element : shape);
  let fields=$derived(resolved.fields ?? (resolved.kind==='array' ? resolved.element?.fields : undefined));
  let type=$derived(goTypes ? shape.type : resolved.kind==='array' ? '[]' : resolved.kind==='struct' || fields ? '{…}' : resolved.kind==='reference' ? '↩' : resolved.kind==='map' ? '{…}' : /^(u?int|float|complex)/.test(resolved.type) ? 'number' : resolved.type==='bool' ? 'boolean' : resolved.type==='string' ? 'string' : resolved.kind==='unknown' ? '?' : resolved.type);
</script>
{#if fields?.length || resolved.element && resolved.kind==='array'}
  <details open={openPaths.includes(path)}>
    <summary onclick={(event)=>{event.preventDefault();ondetail(path,!openPaths.includes(path))}}><span class="key">{name}</span>{#if optional}<span class="optional" title="May be absent">?</span>{/if}<span class="type">{type}</span></summary>
    <div class="children">
      {#if fields?.length}{#each fields as field (field.name)}<Shape name={field.name} shape={field.shape} optional={field.optional} {goTypes} path={path+'/'+escapePath(field.name)} {openPaths} {ondetail}/>{/each}
      {:else if resolved.element}<Shape name="item" shape={resolved.element} {goTypes} path={path+'/item'} {openPaths} {ondetail}/>{/if}
    </div>
  </details>
{:else}
  <div class="field"><span class="key">{name}</span>{#if optional}<span class="optional" title="May be absent">?</span>{/if}<span class="type">{type}</span>{#if resolved.note}<span title={resolved.note} aria-label={resolved.note}>ⓘ</span>{/if}</div>
{/if}
<style>
summary,.field{font-family:ui-monospace,SFMono-Regular,monospace;font-size:13px;line-height:24px;overflow-wrap:anywhere}
summary{cursor:pointer;list-style-position:outside;margin-left:14px}
.field{padding-left:14px}.type{color:#484949;margin-left:12px;font-size:12px}.optional{color:#484949}.children{padding-left:15px;border-left:1px solid #b1b4b6;margin:2px 0 2px 5px}
summary:focus-visible{outline:2px solid #0b0c0c;background:#ffdd00}
</style>
